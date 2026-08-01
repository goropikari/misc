package authz

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Handler exposes the common authorization and administration API.
type Handler struct {
	Store *Store
}

// NewHandler creates an HTTP handler backed by store.
func NewHandler(store *Store) http.Handler {
	return Handler{Store: store}
}

type checkRequest struct {
	Subject            Subject
	Permission         string
	Entitlement        string
	RequestedScope     Scope
	ResourceOwnerID    string
	ResourceDepartment string
	SubjectDepartment  string
}

type roleMutationRequest struct {
	Actor Subject `json:"actor"`
	RoleUpdate
}

type roleCreateRequest struct {
	Actor Subject `json:"actor"`
	Role  Role    `json:"role"`
}

type roleAssignmentRequest struct {
	Actor      Subject  `json:"actor"`
	Assignment UserRole `json:"assignment"`
}

func (h Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // This is the small public route table for the POC.
	if h.Store == nil {
		writeError(writer, http.StatusInternalServerError, "store is not configured")
		return
	}

	path := strings.TrimSuffix(request.URL.Path, "/")
	switch {
	case request.Method == http.MethodGet && path == "/healthz":
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	case strings.HasPrefix(path, "/admin"):
		h.admin(writer, request)
	case strings.HasPrefix(path, "/v1/ext-authz/check") || strings.HasPrefix(path, "/demo-"):
		h.extAuthzCheck(writer, request)
	case request.Method == http.MethodPost && path == "/v1/authz/check":
		h.check(writer, request)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/v1/users/") && strings.HasSuffix(path, "/permissions"):
		h.effectivePermissions(writer, request)
	case request.Method == http.MethodGet && path == "/v1/audit":
		h.audit(writer, request)
	case request.Method == http.MethodPost && path == "/v1/roles":
		h.createRole(writer, request)
	case request.Method == http.MethodPut && strings.HasPrefix(path, "/v1/roles/"):
		h.updateRole(writer, request, strings.TrimPrefix(path, "/v1/roles/"))
	case request.Method == http.MethodPost && path == "/v1/user-roles":
		h.assignRole(writer, request)
	default:
		writeError(writer, http.StatusNotFound, "not found")
	}
}

func (h Handler) extAuthzCheck(writer http.ResponseWriter, request *http.Request) {
	permission := request.Header.Get("x-authz-permission")
	entitlement := request.Header.Get("x-authz-entitlement")

	if permission == "" {
		permission, entitlement = demoAuthorizationMetadata(request.URL.Path, request.Header.Get("path"))
	}

	input := checkRequest{Subject: Subject{
		Type:     SubjectUser,
		ID:       request.Header.Get("x-user-id"),
		TenantID: request.Header.Get("x-tenant-id"),
	}, Permission: permission, Entitlement: entitlement}

	decision := h.Store.Authorizer().Evaluate(AuthorizationRequest{Subject: input.Subject, Permission: input.Permission, Entitlement: input.Entitlement})
	if !decision.Allowed {
		_ = h.Store.AddAudit(AuditLog{TenantID: input.Subject.TenantID, ActorID: input.Subject.ID, Action: "AUTHORIZATION_DENIED", TargetType: "permission", TargetID: input.Permission, Result: "DENIED", Reason: decision.Reason})
		writeJSON(writer, http.StatusForbidden, decision)

		return
	}

	writer.WriteHeader(http.StatusOK)
}

func demoAuthorizationMetadata(paths ...string) (string, string) {
	for _, path := range paths {
		path = strings.TrimPrefix(path, "/v1/ext-authz/check")

		switch {
		case strings.HasPrefix(path, "/demo-a"):
			return "demo-a.dashboard.view", "demo-a.dashboard"
		case strings.HasPrefix(path, "/demo-b"):
			return "demo-b.dashboard.view", "demo-b.dashboard"
		}
	}

	return "", ""
}

func (h Handler) check(writer http.ResponseWriter, request *http.Request) {
	var input checkRequest
	if !decodeJSON(writer, request, &input) {
		return
	}

	decision := h.Store.Authorizer().Evaluate(AuthorizationRequest{Subject: input.Subject, Permission: input.Permission, Entitlement: input.Entitlement, RequestedScope: input.RequestedScope, ResourceOwnerID: input.ResourceOwnerID, ResourceDepartment: input.ResourceDepartment, SubjectDepartment: input.SubjectDepartment})
	if !decision.Allowed {
		_ = h.Store.AddAudit(AuditLog{TenantID: input.Subject.TenantID, ActorID: input.Subject.ID, Action: "AUTHORIZATION_DENIED", TargetType: "permission", TargetID: input.Permission, Result: "DENIED", Reason: decision.Reason})
		writeJSON(writer, http.StatusForbidden, decision)

		return
	}

	writeJSON(writer, http.StatusOK, decision)
}

func (h Handler) effectivePermissions(writer http.ResponseWriter, request *http.Request) {
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[2] == "" {
		writeError(writer, http.StatusBadRequest, "invalid user path")
		return
	}

	subject := Subject{Type: SubjectUser, ID: parts[2], TenantID: request.URL.Query().Get("tenant_id")}
	writeJSON(writer, http.StatusOK, h.Store.Authorizer().EffectivePermissions(subject))
}

func (h Handler) audit(writer http.ResponseWriter, request *http.Request) {
	snapshot := h.Store.Snapshot()
	tenantID := request.URL.Query().Get("tenant_id")

	logs := make([]AuditLog, 0, len(snapshot.AuditLogs))
	for _, log := range snapshot.AuditLogs {
		if tenantID == "" || log.TenantID == tenantID {
			logs = append(logs, log)
		}
	}

	writeJSON(writer, http.StatusOK, logs)
}

func (h Handler) createRole(writer http.ResponseWriter, request *http.Request) {
	var input roleCreateRequest
	if !decodeJSON(writer, request, &input) {
		return
	}

	role, err := h.Store.CreateRole(input.Actor, input.Role)
	if err != nil {
		writeStoreError(writer, err)
		return
	}

	writeJSON(writer, http.StatusCreated, role)
}

func (h Handler) updateRole(writer http.ResponseWriter, request *http.Request, roleID string) {
	var input roleMutationRequest
	if !decodeJSON(writer, request, &input) {
		return
	}

	role, err := h.Store.UpdateRole(input.Actor, roleID, input.RoleUpdate)
	if err != nil {
		writeStoreError(writer, err)
		return
	}

	writeJSON(writer, http.StatusOK, role)
}

func (h Handler) assignRole(writer http.ResponseWriter, request *http.Request) {
	var input roleAssignmentRequest
	if !decodeJSON(writer, request, &input) {
		return
	}

	if err := h.Store.AssignRole(input.Actor, input.Assignment); err != nil {
		writeStoreError(writer, err)
		return
	}

	writeJSON(writer, http.StatusCreated, input.Assignment)
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON")
		return false
	}

	return true
}

func writeStoreError(writer http.ResponseWriter, err error) {
	status := http.StatusBadRequest

	switch {
	case errors.Is(err, ErrRoleNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrVersionConflict):
		status = http.StatusConflict
	case errors.Is(err, ErrPermissionDenied):
		status = http.StatusForbidden
	}

	writeError(writer, status, err.Error())
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
