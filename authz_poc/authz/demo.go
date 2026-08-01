package authz

// SeedDemo installs the two datasets used by the compose demo. It is
// idempotent so restarting the authorization service does not change state.
func (s *Store) SeedDemo() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data.Tenants["demo-tenant"]; exists {
		return nil
	}

	s.data.Tenants["demo-tenant"] = Tenant{ID: "demo-tenant", Active: true, Members: map[string]bool{"demo-user": true}, Entitlements: []TenantEntitlement{
		{Key: "demo-a.dashboard", Enabled: true}, {Key: "demo-b.dashboard", Enabled: true},
	}}

	for _, product := range []string{"demo-a", "demo-b"} {
		permission := product + ".dashboard.view"
		entitlement := product + ".dashboard"
		roleID := product + "-viewer"
		s.data.Entitlements[entitlement] = Entitlement{Key: entitlement, Permissions: []string{permission}}
		s.data.Permissions[permission] = Permission{Key: permission, Product: product, Resource: "dashboard", Action: "view", Active: true}
		s.data.Roles[roleID] = Role{ID: roleID, TenantID: "demo-tenant", Name: product + " Viewer", Active: true, Permissions: []RolePermission{{Permission: permission, Configured: true, Scope: ScopeAll}}}
		s.data.UserRoles = append(s.data.UserRoles, UserRole{UserID: "demo-user", RoleID: roleID, Valid: true})
	}

	s.data.AuthorizationVersion++

	return s.persistLocked()
}
