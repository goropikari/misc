//nolint:testpackage // HTTP tests need to seed the in-memory POC snapshot.
package authz

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerServeHTTP(t *testing.T) {
	t.Run("when product permissions page is requested: groups permissions and shows tenant status", func(t *testing.T) {
		// Arrange
		store := NewMemoryStore()
		store.data.Tenants["tenant-1"] = Tenant{ID: "tenant-1", Entitlements: []TenantEntitlement{{Key: "billing.basic", Enabled: true}}}
		store.data.Entitlements["billing.basic"] = Entitlement{Key: "billing.basic", Permissions: []string{"invoice.read"}}
		store.data.Permissions["invoice.read"] = Permission{Key: "invoice.read", Product: "billing", Resource: "invoice", Action: "read", Active: true, CustomRoleAssignable: true}
		store.data.Permissions["invoice.approve"] = Permission{Key: "invoice.approve", Product: "billing", Resource: "invoice", Action: "approve", Active: true, CustomRoleAssignable: false, Requires: []string{"invoice.read"}}
		store.data.Roles["role-1"] = Role{ID: "role-1", TenantID: "tenant-1", Permissions: []RolePermission{{Permission: "invoice.read", Configured: true}}}
		request := httptest.NewRequest(http.MethodGet, "/admin/permissions?tenant_id=tenant-1&product=billing", nil)
		response := httptest.NewRecorder()

		// Act
		NewHandler(store).ServeHTTP(response, request)

		// Assert
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "権限管理")
		assert.Contains(t, response.Body.String(), "invoice.read")
		assert.Contains(t, response.Body.String(), "契約中")
		assert.Contains(t, response.Body.String(), "invoice.approve")
		assert.Contains(t, response.Body.String(), "未契約")
		assert.Contains(t, response.Body.String(), "設定ロール数")
	})

	t.Run("when admin roles page is requested: renders role management UI", func(t *testing.T) {
		// Arrange
		store := NewMemoryStore()
		store.data.Roles["role-1"] = Role{ID: "role-1", TenantID: "tenant-1", Name: "経理担当", Active: true}
		request := httptest.NewRequest(http.MethodGet, "/admin/roles?tenant_id=tenant-1", nil)
		response := httptest.NewRecorder()

		// Act
		NewHandler(store).ServeHTTP(response, request)

		// Assert
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "ロール管理")
		assert.Contains(t, response.Body.String(), "経理担当")
	})

	t.Run("when check is allowed: returns decision without leaking internal error", func(t *testing.T) {
		// Arrange
		store := NewMemoryStore()
		store.data.Tenants["tenant-1"] = Tenant{ID: "tenant-1", Active: true, Members: map[string]bool{"user-1": true}, Entitlements: []TenantEntitlement{{Key: "billing", Enabled: true}}}
		store.data.Entitlements["billing"] = Entitlement{Key: "billing", Permissions: []string{"invoice.read"}}
		store.data.Roles["role-1"] = Role{ID: "role-1", TenantID: "tenant-1", Active: true, Permissions: []RolePermission{{Permission: "invoice.read", Configured: true}}}
		store.data.UserRoles = []UserRole{{UserID: "user-1", RoleID: "role-1", Valid: true}}
		body, err := json.Marshal(checkRequest{Subject: Subject{Type: SubjectUser, ID: "user-1", TenantID: "tenant-1"}, Permission: "invoice.read", Entitlement: "billing"})
		require.NoError(t, err)

		request := httptest.NewRequest(http.MethodPost, "/v1/authz/check", bytes.NewReader(body))
		response := httptest.NewRecorder()

		// Act
		NewHandler(store).ServeHTTP(response, request)

		// Assert
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"Allowed":true`)
	})

	t.Run("when check is denied: returns forbidden and records audit", func(t *testing.T) {
		// Arrange
		store := NewMemoryStore()
		body := bytes.NewBufferString(`{"subject":{"Type":"user","ID":"user-1","TenantID":"missing"},"permission":"invoice.read"}`)
		request := httptest.NewRequest(http.MethodPost, "/v1/authz/check", body)
		response := httptest.NewRecorder()

		// Act
		NewHandler(store).ServeHTTP(response, request)

		// Assert
		assert.Equal(t, http.StatusForbidden, response.Code)
		require.Len(t, store.Snapshot().AuditLogs, 1)
		assert.Equal(t, ReasonTenantNotFound, store.Snapshot().AuditLogs[0].Reason)
	})
}
