package authz

import (
	"slices"
	"sort"
)

// Authorizer evaluates common authorization rules against an in-memory snapshot.
// Replacing the snapshot on each request is enough to model immediate updates in
// this POC and avoids embedding permissions in a long-lived token.
type Authorizer struct {
	Tenants      map[string]Tenant
	Entitlements map[string]Entitlement
	Permissions  map[string]Permission
	Roles        map[string]Role
	UserRoles    []UserRole
	Services     map[string]map[string]bool
}

// Evaluate applies the common authorization checks in PRD order.
func (a Authorizer) Evaluate(req AuthorizationRequest) Decision { //nolint:cyclop,gocognit // The ordered checks mirror the authorization equation in the PRD.
	if req.Subject.ID == "" || req.Subject.Type == "" {
		return Decision{Reason: ReasonAuthenticationRequired}
	}

	if req.Permission == "" {
		return Decision{Reason: ReasonInvalidRequest}
	}

	if req.Subject.Type == SubjectService {
		if a.Services[req.Subject.ID][req.Permission] {
			return Decision{Allowed: true, Reason: ReasonAllowed}
		}

		return Decision{Reason: ReasonServiceAccessDenied}
	}

	tenant, ok := a.Tenants[req.Subject.TenantID]
	if !ok {
		return Decision{Reason: ReasonTenantNotFound}
	}

	if !tenant.Active {
		return Decision{Reason: ReasonTenantSuspended}
	}

	if !tenant.Members[req.Subject.ID] {
		return Decision{Reason: ReasonTenantMembershipMissing}
	}

	if req.Entitlement != "" && !enabledEntitlement(tenant, req.Entitlement) {
		return Decision{Reason: ReasonEntitlementMissing}
	}

	if req.Entitlement != "" && !entitlementIncludes(a.Entitlements[req.Entitlement], req.Permission) {
		return Decision{Reason: ReasonEntitlementMissing}
	}

	decision := a.permissionDecision(req)
	if decision.Allowed && !scopeAllowed(req, decision.Scope) {
		decision.Allowed = false
		decision.Reason = ReasonScopeDenied
	}

	return decision
}

func (a Authorizer) permissionDecision(req AuthorizationRequest) Decision { //nolint:cyclop,gocognit // Role and permission aggregation is intentionally kept in one evaluation step.
	var (
		roleIDs  []string
		selected Scope
	)

	roleFound := false
	activeRoleFound := false

	for _, assignment := range a.UserRoles {
		if assignment.UserID != req.Subject.ID || !assignment.Valid {
			continue
		}

		role, ok := a.Roles[assignment.RoleID]
		if !ok || role.TenantID != req.Subject.TenantID {
			continue
		}

		roleFound = true

		if !role.Active {
			continue
		}

		activeRoleFound = true

		for _, permission := range role.Permissions {
			if permission.Configured && permission.Permission == req.Permission {
				roleIDs = append(roleIDs, role.ID)
				selected = strongerScope(selected, permission.Scope)
			}
		}
	}

	if len(roleIDs) == 0 {
		if roleFound && !activeRoleFound {
			return Decision{Reason: ReasonRoleDisabled}
		}

		return Decision{Reason: ReasonPermissionMissing}
	}

	sort.Strings(roleIDs)

	return Decision{Allowed: true, Reason: ReasonAllowed, RoleIDs: roleIDs, Scope: selected}
}

// EffectivePermissions returns configured permissions, including those disabled
// by a currently missing entitlement.
func (a Authorizer) EffectivePermissions(subject Subject) []EffectivePermission { //nolint:cyclop,gocognit // The report intentionally preserves all reasons needed by the UI.
	permissions := make(map[string]*EffectivePermission)

	for _, assignment := range a.UserRoles {
		if assignment.UserID != subject.ID || !assignment.Valid {
			continue
		}

		role, ok := a.Roles[assignment.RoleID]
		if !ok || role.TenantID != subject.TenantID {
			continue
		}

		for _, configured := range role.Permissions {
			if !configured.Configured {
				continue
			}

			item := permissions[configured.Permission]
			if item == nil {
				item = &EffectivePermission{Permission: configured.Permission, Configured: true, Effective: role.Active, Reason: ReasonAllowed, Scope: configured.Scope}
				permissions[configured.Permission] = item
			}

			item.RoleIDs = append(item.RoleIDs, role.ID)

			item.Scope = strongerScope(item.Scope, configured.Scope)
			if role.Active {
				item.Effective = true
			}
		}
	}

	tenant := a.Tenants[subject.TenantID]
	for _, item := range permissions {
		if !a.permissionLicensed(tenant, item.Permission) {
			item.Effective = false
			item.Reason = ReasonEntitlementMissing
		}

		if !item.Effective && item.Reason == ReasonAllowed {
			item.Reason = ReasonRoleDisabled
		}

		sort.Strings(item.RoleIDs)
	}

	result := make([]EffectivePermission, 0, len(permissions))
	for _, item := range permissions {
		result = append(result, *item)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Permission < result[j].Permission })

	return result
}

func (a Authorizer) permissionLicensed(tenant Tenant, permission string) bool {
	for key, entitlement := range a.Entitlements {
		if entitlementIncludes(entitlement, permission) && enabledEntitlement(tenant, key) {
			return true
		}
	}

	return false
}

func enabledEntitlement(tenant Tenant, key string) bool {
	for _, entitlement := range tenant.Entitlements {
		if entitlement.Key == key {
			return entitlement.Enabled
		}
	}

	return false
}

func entitlementIncludes(entitlement Entitlement, permission string) bool {
	return slices.Contains(entitlement.Permissions, permission)
}

func strongerScope(current, candidate Scope) Scope {
	if current == "" {
		return candidate
	}

	if candidate == ScopeAll || current == ScopeAll {
		return ScopeAll
	}

	return current
}

func scopeAllowed(req AuthorizationRequest, granted Scope) bool { //nolint:cyclop // Scope hierarchy is explicit for this small POC.
	if req.RequestedScope == "" || granted == "" || granted == ScopeAll {
		return true
	}

	if req.RequestedScope == granted {
		return true
	}

	if granted == ScopeOrganization {
		return req.RequestedScope == ScopeDepartment || req.RequestedScope == ScopeTeam || req.RequestedScope == ScopeOwn
	}

	if granted == ScopeDepartment {
		return req.RequestedScope == ScopeTeam || req.RequestedScope == ScopeOwn
	}

	if granted == ScopeTeam {
		return req.RequestedScope == ScopeOwn
	}

	return false
}
