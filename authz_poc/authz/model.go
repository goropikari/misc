// Package authz contains the in-memory authorization model used by the POC.
package authz

import "time"

// SubjectType identifies the kind of principal making a request.
type SubjectType string

const (
	SubjectUser    SubjectType = "user"
	SubjectService SubjectType = "service"
)

// Subject is the authenticated principal.
type Subject struct {
	Type     SubjectType
	ID       string
	TenantID string
}

// Scope is the data range attached to a role permission.
type Scope string

const (
	ScopeOwn          Scope = "own"
	ScopeTeam         Scope = "team"
	ScopeDepartment   Scope = "department"
	ScopeOrganization Scope = "organization"
	ScopeAll          Scope = "all"
)

// RolePermission is a permission configured on a role. Configured remains true
// after an entitlement is removed so that a later re-subscription restores it.
type RolePermission struct {
	Permission string
	Scope      Scope
	Configured bool
}

// Role is either a system role or a tenant-owned custom role.
type Role struct {
	ID          string
	TenantID    string
	Name        string
	System      bool
	Active      bool
	Permissions []RolePermission
	Version     int
}

// Permission describes a permission registered by a product.
type Permission struct {
	Key                  string
	Product              string
	Resource             string
	Action               string
	Delegatable          bool
	CustomRoleAssignable bool
	Active               bool
	Requires             []string
}

// Entitlement describes a product capability and the permissions it enables.
type Entitlement struct {
	Key         string
	Permissions []string
}

// TenantEntitlement is an entitlement assigned to a tenant.
type TenantEntitlement struct {
	Key     string
	Enabled bool
}

// UserRole assigns a role to a user within one tenant.
type UserRole struct {
	UserID string
	RoleID string
	Valid  bool
}

// DelegatablePermission limits what a subject may grant to another role.
type DelegatablePermission struct {
	TenantID   string
	SubjectID  string
	Permission string
	MaxScope   Scope
}

// AuditLog records a state-changing or denied authorization operation.
type AuditLog struct {
	ID         string
	TenantID   string
	ActorID    string
	Action     string
	TargetType string
	TargetID   string
	Before     any
	After      any
	Result     string
	Reason     DecisionReason
	CreatedAt  time.Time
}

// Tenant is the tenant-side context needed by the common authorization layer.
type Tenant struct {
	ID           string
	Active       bool
	Members      map[string]bool
	Entitlements []TenantEntitlement
}

// AuthorizationRequest describes the metadata normally supplied by Envoy.
type AuthorizationRequest struct {
	Subject            Subject
	Permission         string
	Entitlement        string
	RequestedScope     Scope
	ResourceOwnerID    string
	ResourceDepartment string
	SubjectDepartment  string
	Now                time.Time
}

// DecisionReason is a machine-readable internal reason for a decision.
type DecisionReason string

const (
	ReasonAllowed                 DecisionReason = "ALLOW"
	ReasonAuthenticationRequired  DecisionReason = "AUTHENTICATION_REQUIRED"
	ReasonTenantNotFound          DecisionReason = "TENANT_NOT_FOUND"
	ReasonTenantSuspended         DecisionReason = "TENANT_SUSPENDED"
	ReasonTenantMembershipMissing DecisionReason = "TENANT_MEMBERSHIP_MISSING"
	ReasonEntitlementMissing      DecisionReason = "ENTITLEMENT_MISSING"
	ReasonPermissionMissing       DecisionReason = "PERMISSION_MISSING"
	ReasonRoleDisabled            DecisionReason = "ROLE_DISABLED"
	ReasonScopeDenied             DecisionReason = "SCOPE_DENIED"
	ReasonServiceAccessDenied     DecisionReason = "SERVICE_ACCESS_DENIED"
	ReasonInvalidRequest          DecisionReason = "INVALID_REQUEST"
)

// Decision is the internal authorization result. External adapters can expose
// Allowed as a generic 403 while retaining Reason for logs and diagnostics.
type Decision struct {
	Allowed bool
	Reason  DecisionReason
	RoleIDs []string
	Scope   Scope
}

// RoleUpdate is the API payload for updating a custom role.
type RoleUpdate struct {
	Name        string
	Description string
	Permissions []RolePermission
	Version     int
}

// AuthorizationSnapshot is the persisted POC state.
type AuthorizationSnapshot struct {
	Tenants                map[string]Tenant
	Entitlements           map[string]Entitlement
	Permissions            map[string]Permission
	Roles                  map[string]Role
	UserRoles              []UserRole
	Services               map[string]map[string]bool
	DelegatablePermissions []DelegatablePermission
	AuthorizationVersion   uint64
	AuditLogs              []AuditLog
}

// EffectivePermission describes one permission for an effective-permissions UI.
type EffectivePermission struct {
	Permission string
	Configured bool
	Effective  bool
	Reason     DecisionReason
	RoleIDs    []string
	Scope      Scope
}
