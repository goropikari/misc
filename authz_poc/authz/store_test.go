//nolint:testpackage // Store tests need to seed the in-memory POC snapshot.
package authz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreUpdateRole(t *testing.T) {
	t.Run("when actor can delegate and version matches: updates role and audits it", func(t *testing.T) {
		// Arrange
		store := NewMemoryStore()
		store.data.Tenants["tenant-1"] = Tenant{ID: "tenant-1", Active: true, Members: map[string]bool{"admin": true}}
		store.data.Roles["role-1"] = Role{ID: "role-1", TenantID: "tenant-1", Name: "old", Version: 3}
		store.data.Permissions["invoice.read"] = Permission{Key: "invoice.read", Active: true, CustomRoleAssignable: true}
		store.data.DelegatablePermissions = []DelegatablePermission{{TenantID: "tenant-1", SubjectID: "admin", Permission: "invoice.read", MaxScope: ScopeDepartment}}

		// Act
		role, err := store.UpdateRole(Subject{Type: SubjectUser, ID: "admin", TenantID: "tenant-1"}, "role-1", RoleUpdate{Name: "new", Version: 3, Permissions: []RolePermission{{Permission: "invoice.read", Configured: true}}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "new", role.Name)
		assert.Equal(t, 4, role.Version)
		assert.Equal(t, uint64(1), store.Snapshot().AuthorizationVersion)
		require.Len(t, store.Snapshot().AuditLogs, 1)
		assert.Equal(t, "ROLE_UPDATED", store.Snapshot().AuditLogs[0].Action)
	})

	t.Run("when version is stale: rejects update without changing state", func(t *testing.T) {
		// Arrange
		store := NewMemoryStore()
		store.data.Tenants["tenant-1"] = Tenant{ID: "tenant-1", Members: map[string]bool{"admin": true}}
		store.data.Roles["role-1"] = Role{ID: "role-1", TenantID: "tenant-1", Name: "old", Version: 2}

		// Act
		_, err := store.UpdateRole(Subject{Type: SubjectUser, ID: "admin", TenantID: "tenant-1"}, "role-1", RoleUpdate{Name: "new", Version: 1})

		// Assert
		require.ErrorIs(t, err, ErrVersionConflict)
		assert.Equal(t, "old", store.Snapshot().Roles["role-1"].Name)
		assert.Empty(t, store.Snapshot().AuditLogs)
	})

	t.Run("when dependency is missing: rejects privilege escalation", func(t *testing.T) {
		// Arrange
		store := NewMemoryStore()
		store.data.Tenants["tenant-1"] = Tenant{ID: "tenant-1", Members: map[string]bool{"admin": true}}
		store.data.Roles["role-1"] = Role{ID: "role-1", TenantID: "tenant-1", Version: 1}
		store.data.Permissions["invoice.approve"] = Permission{Key: "invoice.approve", Active: true, CustomRoleAssignable: true, Requires: []string{"invoice.read"}}
		store.data.DelegatablePermissions = []DelegatablePermission{{TenantID: "tenant-1", SubjectID: "admin", Permission: "invoice.approve", MaxScope: ScopeAll}}

		// Act
		_, err := store.UpdateRole(Subject{Type: SubjectUser, ID: "admin", TenantID: "tenant-1"}, "role-1", RoleUpdate{Name: "approver", Version: 1, Permissions: []RolePermission{{Permission: "invoice.approve", Configured: true}}})

		// Assert
		assert.ErrorIs(t, err, ErrDependencyMissing)
	})
}
