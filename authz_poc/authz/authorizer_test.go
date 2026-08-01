//nolint:testpackage // The fixture intentionally uses the in-memory model directly.
package authz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizerEvaluate(t *testing.T) {
	t.Run("when entitlement and role both grant permission: allows request", func(t *testing.T) {
		// Arrange
		authorizer := sampleAuthorizer()
		request := AuthorizationRequest{Subject: Subject{Type: SubjectUser, ID: "user-1", TenantID: "tenant-1"}, Entitlement: "billing.approval", Permission: "invoice.approve"}

		// Act
		decision := authorizer.Evaluate(request)

		// Assert
		assert.True(t, decision.Allowed)
		assert.Equal(t, ReasonAllowed, decision.Reason)
		assert.Equal(t, []string{"role-1"}, decision.RoleIDs)
	})

	t.Run("when entitlement is missing: denies while role configuration remains", func(t *testing.T) {
		// Arrange
		authorizer := sampleAuthorizer()
		authorizer.Tenants["tenant-1"] = Tenant{ID: "tenant-1", Active: true, Members: map[string]bool{"user-1": true}}
		request := AuthorizationRequest{Subject: Subject{Type: SubjectUser, ID: "user-1", TenantID: "tenant-1"}, Entitlement: "billing.approval", Permission: "invoice.approve"}

		// Act
		decision := authorizer.Evaluate(request)

		// Assert
		assert.False(t, decision.Allowed)
		assert.Equal(t, ReasonEntitlementMissing, decision.Reason)

		effective := authorizer.EffectivePermissions(request.Subject)
		require.Len(t, effective, 2)
		assert.Equal(t, "invoice.approve", effective[0].Permission)
		assert.False(t, effective[0].Effective)
		assert.Equal(t, ReasonEntitlementMissing, effective[0].Reason)
		assert.True(t, effective[0].Configured)
	})

	t.Run("when user has multiple roles: unions permissions", func(t *testing.T) {
		// Arrange
		authorizer := sampleAuthorizer()
		authorizer.Roles["role-2"] = Role{ID: "role-2", TenantID: "tenant-1", Name: "Viewer", Active: true, Permissions: []RolePermission{{Permission: "report.export", Configured: true}}}
		authorizer.UserRoles = append(authorizer.UserRoles, UserRole{UserID: "user-1", RoleID: "role-2", Valid: true})

		// Act
		decision := authorizer.Evaluate(AuthorizationRequest{Subject: Subject{Type: SubjectUser, ID: "user-1", TenantID: "tenant-1"}, Entitlement: "reporting", Permission: "report.export"})

		// Assert
		assert.True(t, decision.Allowed)
		assert.Equal(t, []string{"role-2"}, decision.RoleIDs)
	})

	t.Run("when service lacks service permission: denies without tenant lookup", func(t *testing.T) {
		// Arrange
		authorizer := sampleAuthorizer()

		// Act
		decision := authorizer.Evaluate(AuthorizationRequest{Subject: Subject{Type: SubjectService, ID: "billing-service"}, Permission: "customer.read_internal"})

		// Assert
		assert.False(t, decision.Allowed)
		assert.Equal(t, ReasonServiceAccessDenied, decision.Reason)
	})

	t.Run("when requested scope exceeds granted scope: denies request", func(t *testing.T) {
		// Arrange
		authorizer := sampleAuthorizer()

		// Act
		decision := authorizer.Evaluate(AuthorizationRequest{Subject: Subject{Type: SubjectUser, ID: "user-1", TenantID: "tenant-1"}, Entitlement: "billing.approval", Permission: "invoice.approve", RequestedScope: ScopeAll})

		// Assert
		assert.False(t, decision.Allowed)
		assert.Equal(t, ReasonScopeDenied, decision.Reason)
	})
}

func TestAuthorizerEffectivePermissions(t *testing.T) {
	t.Run("when role is disabled: reports configured permission as inactive", func(t *testing.T) {
		// Arrange
		authorizer := sampleAuthorizer()
		authorizer.Roles["role-1"] = Role{ID: "role-1", TenantID: "tenant-1", Name: "Approver", Active: false, Permissions: []RolePermission{{Permission: "invoice.approve", Configured: true}}}

		// Act
		permissions := authorizer.EffectivePermissions(Subject{Type: SubjectUser, ID: "user-1", TenantID: "tenant-1"})

		// Assert
		require.Len(t, permissions, 1)
		assert.Equal(t, ReasonRoleDisabled, permissions[0].Reason)
		assert.False(t, permissions[0].Effective)
	})
}

func sampleAuthorizer() Authorizer {
	return Authorizer{
		Tenants:      map[string]Tenant{"tenant-1": {ID: "tenant-1", Active: true, Members: map[string]bool{"user-1": true}, Entitlements: []TenantEntitlement{{Key: "billing.approval", Enabled: true}, {Key: "reporting", Enabled: true}}}},
		Entitlements: map[string]Entitlement{"billing.approval": {Key: "billing.approval", Permissions: []string{"invoice.read", "invoice.approve"}}, "reporting": {Key: "reporting", Permissions: []string{"report.export"}}},
		Roles:        map[string]Role{"role-1": {ID: "role-1", TenantID: "tenant-1", Name: "Approver", Active: true, Permissions: []RolePermission{{Permission: "invoice.read", Scope: ScopeDepartment, Configured: true}, {Permission: "invoice.approve", Scope: ScopeDepartment, Configured: true}}}},
		UserRoles:    []UserRole{{UserID: "user-1", RoleID: "role-1", Valid: true}},
		Services:     map[string]map[string]bool{"billing-service": {"invoice.read_internal": true}},
	}
}
