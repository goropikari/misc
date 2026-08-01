package authz

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	ErrRoleNotFound       = errors.New("role not found")
	ErrRoleImmutable      = errors.New("system roles cannot be changed")
	ErrVersionConflict    = errors.New("role version conflict")
	ErrPermissionUnknown  = errors.New("permission is unknown")
	ErrPermissionInactive = errors.New("permission is inactive")
	ErrPermissionDenied   = errors.New("permission cannot be assigned by actor")
	ErrDependencyMissing  = errors.New("permission dependency is missing")
	ErrScopeTooBroad      = errors.New("scope exceeds actor delegation")
	ErrInvalidRole        = errors.New("invalid role")
	ErrRoleExists         = errors.New("role already exists")
)

// Store is a concurrency-safe PostgreSQL-backed authorization store.
type Store struct {
	mu   sync.RWMutex
	db   *sql.DB
	data AuthorizationSnapshot
}

// NewMemoryStore creates a store without persistence. It is intended for unit
// tests and local examples; production should use NewPostgresStore.
func NewMemoryStore() *Store {
	return &Store{data: emptySnapshot()}
}

// NewPostgresStore connects to PostgreSQL and loads the single POC snapshot.
// PostgreSQL 17.7 is provided by the repository's compose file.
func NewPostgresStore(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres authorization store: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres authorization store: %w", err)
	}

	if _, err := db.Exec(postgresSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize postgres authorization store: %w", err)
	}

	store := &Store{db: db, data: emptySnapshot()}

	var payload []byte

	err = db.QueryRow(`SELECT payload FROM authorization_state WHERE id = 1`).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		if err := store.persistLocked(); err != nil {
			_ = db.Close()
			return nil, err
		}

		return store, nil
	}

	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("load postgres authorization store: %w", err)
	}

	if err := json.Unmarshal(payload, &store.data); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("decode postgres authorization store: %w", err)
	}

	return store, nil
}

// Close releases the PostgreSQL connection. It is a no-op for memory stores.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}

	return s.db.Close()
}

func emptySnapshot() AuthorizationSnapshot {
	return AuthorizationSnapshot{Tenants: map[string]Tenant{}, Entitlements: map[string]Entitlement{}, Permissions: map[string]Permission{}, Roles: map[string]Role{}, Services: map[string]map[string]bool{}}
}

// Snapshot returns a copy of the current state for authorization evaluation.
func (s *Store) Snapshot() AuthorizationSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data
}

// Authorizer builds an evaluator from the current state.
func (s *Store) Authorizer() Authorizer {
	d := s.Snapshot()
	return Authorizer{Tenants: d.Tenants, Entitlements: d.Entitlements, Roles: d.Roles, UserRoles: d.UserRoles, Services: d.Services, Permissions: d.Permissions}
}

// UpdateRole validates and persists a custom role update atomically.
func (s *Store) UpdateRole(actor Subject, roleID string, update RoleUpdate) (Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, ok := s.data.Roles[roleID]
	if !ok {
		return Role{}, ErrRoleNotFound
	}

	if role.System {
		return Role{}, ErrRoleImmutable
	}

	if role.TenantID != actor.TenantID || actor.Type != SubjectUser || !s.data.Tenants[actor.TenantID].Members[actor.ID] {
		return Role{}, ErrPermissionDenied
	}

	if role.Version != update.Version {
		return Role{}, ErrVersionConflict
	}

	if update.Name == "" {
		return Role{}, ErrInvalidRole
	}

	if err := s.validateRolePermissions(actor, role.TenantID, update.Permissions); err != nil {
		return Role{}, err
	}

	before := role
	role.Name = update.Name
	role.Permissions = append([]RolePermission(nil), update.Permissions...)
	role.Version++
	s.data.Roles[roleID] = role
	s.data.AuthorizationVersion++
	s.appendAuditLocked(AuditLog{TenantID: actor.TenantID, ActorID: actor.ID, Action: "ROLE_UPDATED", TargetType: "role", TargetID: roleID, Before: before, After: role, Result: "SUCCESS", Reason: ReasonAllowed})

	if err := s.persistLocked(); err != nil {
		return Role{}, err
	}

	return role, nil
}

// CreateRole creates a tenant-owned custom role after applying the same
// permission delegation rules as updates.
func (s *Store) CreateRole(actor Subject, role Role) (Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if role.ID == "" || role.Name == "" || role.System || role.TenantID != actor.TenantID {
		return Role{}, ErrInvalidRole
	}

	if _, exists := s.data.Roles[role.ID]; exists {
		return Role{}, ErrRoleExists
	}

	if actor.Type != SubjectUser || !s.data.Tenants[actor.TenantID].Members[actor.ID] {
		return Role{}, ErrPermissionDenied
	}

	if err := s.validateRolePermissions(actor, role.TenantID, role.Permissions); err != nil {
		return Role{}, err
	}

	role.Active = true
	role.Version = 1
	s.data.Roles[role.ID] = role
	s.data.AuthorizationVersion++
	s.appendAuditLocked(AuditLog{TenantID: actor.TenantID, ActorID: actor.ID, Action: "ROLE_CREATED", TargetType: "role", TargetID: role.ID, After: role, Result: "SUCCESS", Reason: ReasonAllowed})

	if err := s.persistLocked(); err != nil {
		return Role{}, err
	}

	return role, nil
}

// AssignRole assigns a role to a user in the same tenant.
func (s *Store) AssignRole(actor Subject, assignment UserRole) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, ok := s.data.Roles[assignment.RoleID]
	if !ok {
		return ErrRoleNotFound
	}

	if actor.Type != SubjectUser || actor.TenantID != role.TenantID || !s.data.Tenants[actor.TenantID].Members[actor.ID] {
		return ErrPermissionDenied
	}

	if assignment.UserID == "" || !s.data.Tenants[actor.TenantID].Members[assignment.UserID] {
		return ErrPermissionDenied
	}

	assignment.Valid = true
	s.data.UserRoles = append(s.data.UserRoles, assignment)
	s.data.AuthorizationVersion++
	s.appendAuditLocked(AuditLog{TenantID: actor.TenantID, ActorID: actor.ID, Action: "USER_ROLE_ADDED", TargetType: "user_role", TargetID: assignment.UserID + ":" + assignment.RoleID, After: assignment, Result: "SUCCESS", Reason: ReasonAllowed})

	return s.persistLocked()
}

func (s *Store) validateRolePermissions(actor Subject, tenantID string, requested []RolePermission) error {
	seen := make(map[string]bool)
	for _, item := range requested {
		if seen[item.Permission] {
			return fmt.Errorf("%w: %s", ErrInvalidRole, item.Permission)
		}

		seen[item.Permission] = true

		permission, ok := s.data.Permissions[item.Permission]
		if !ok {
			return fmt.Errorf("%w: %s", ErrPermissionUnknown, item.Permission)
		}

		if !permission.Active {
			return fmt.Errorf("%w: %s", ErrPermissionInactive, item.Permission)
		}

		if !permission.CustomRoleAssignable {
			return fmt.Errorf("%w: %s", ErrPermissionDenied, item.Permission)
		}

		if !s.canDelegate(actor, tenantID, item.Permission, item.Scope) {
			return fmt.Errorf("%w: %s", ErrPermissionDenied, item.Permission)
		}

		for _, required := range permission.Requires {
			if !seen[required] {
				return fmt.Errorf("%w: %s requires %s", ErrDependencyMissing, item.Permission, required)
			}
		}
	}

	return nil
}

func (s *Store) canDelegate(actor Subject, tenantID string, permission string, requested Scope) bool {
	for _, grant := range s.data.DelegatablePermissions {
		if grant.TenantID == tenantID && grant.SubjectID == actor.ID && grant.Permission == permission && scopeAllowed(AuthorizationRequest{RequestedScope: requested}, grant.MaxScope) {
			return true
		}
	}

	return false
}

// AddAudit records an audit event and persists it.
func (s *Store) AddAudit(log AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.appendAuditLocked(log)

	return s.persistLocked()
}

func (s *Store) appendAuditLocked(log AuditLog) {
	if log.ID == "" {
		log.ID = randomID()
	}

	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}

	s.data.AuditLogs = append(s.data.AuditLogs, log)
}

func randomID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("audit-%d", time.Now().UnixNano())
	}

	return hex.EncodeToString(bytes)
}

func (s *Store) persistLocked() error {
	if s.db != nil {
		return s.persistPostgresLocked()
	}

	return nil
}

func (s *Store) persistPostgresLocked() error {
	contents, err := json.Marshal(s.data)
	if err != nil {
		return fmt.Errorf("encode authorization store: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin authorization transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`INSERT INTO authorization_state (id, payload, authorization_version, updated_at)
VALUES (1, $1, $2, now())
ON CONFLICT (id) DO UPDATE SET payload = EXCLUDED.payload,
authorization_version = EXCLUDED.authorization_version, updated_at = now()`, contents, s.data.AuthorizationVersion)
	if err != nil {
		return fmt.Errorf("persist authorization transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit authorization transaction: %w", err)
	}

	return nil
}

const postgresSchema = `
CREATE TABLE IF NOT EXISTS authorization_state (
    id integer PRIMARY KEY CHECK (id = 1),
    payload jsonb NOT NULL,
    authorization_version bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now()
);
`
