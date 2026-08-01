package authz

import (
	"html/template"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
)

var adminTemplate = template.Must(template.New("admin").Parse(`<!doctype html>
<html lang="ja">
<head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} - Authorization Admin</title>
<style>
:root { color-scheme: light; font-family: system-ui, sans-serif; }
body { background:#f6f7fb; color:#20242b; margin:0; }
header { background:#202a44; color:white; padding:18px 28px; }
header a { color:white; text-decoration:none; margin-right:20px; }
main { max-width:1100px; margin:28px auto; padding:0 20px; }
section, table, form { background:white; border:1px solid #e1e5ee; border-radius:8px; padding:18px; margin:16px 0; }
table { width:100%; border-collapse:collapse; padding:0; }
th, td { text-align:left; padding:11px; border-bottom:1px solid #edf0f5; }
th { color:#596273; font-size:.9rem; }
label { display:block; font-weight:600; margin:12px 0 5px; }
input, textarea, select { box-sizing:border-box; width:100%; padding:9px; border:1px solid #cbd2df; border-radius:5px; }
textarea { min-height:130px; font-family:monospace; }
button, .button { display:inline-block; background:#315dcc; color:white; border:0; border-radius:5px; padding:10px 15px; text-decoration:none; cursor:pointer; }
.muted { color:#687386; } .ok { color:#16834a; font-weight:600; } .bad { color:#b42318; font-weight:600; }
.notice { background:#fff4d6; border:1px solid #f0d27a; padding:12px; border-radius:5px; }
</style></head>
<body><header><strong>Authorization Admin</strong>
<nav style="display:inline"><a href="/admin/permissions?tenant_id={{.TenantID}}">権限</a><a href="/admin/roles?tenant_id={{.TenantID}}">ロール</a><a href="/admin/audit?tenant_id={{.TenantID}}">監査ログ</a></nav></header>
<main><h1>{{.Title}}</h1>{{if .Message}}<p class="notice">{{.Message}}</p>{{end}}{{template "content" .}}</main>
</body></html>`))

type adminPage struct {
	Title       string
	TenantID    string
	Message     string
	Roles       []Role
	Role        Role
	Rows        []adminPermissionRow
	Products    []string
	Product     string
	ProductRows []adminProductPermissionRow
	UserID      string
	Audit       []AuditLog
}

type adminPermissionRow struct {
	Permission string
	Effective  bool
	Configured bool
	Reason     DecisionReason
	Scope      Scope
	RoleIDs    string
}

type adminProductPermissionRow struct {
	Key             string
	Resource        string
	Action          string
	Licensed        bool
	Active          bool
	Assignable      bool
	ConfiguredRoles int
	Requires        string
}

func (h Handler) admin(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // The admin route table is intentionally explicit.
	if request.Method == http.MethodGet && request.URL.Path == "/admin/permissions" {
		h.adminPermissions(writer, request)
		return
	}

	if request.Method == http.MethodGet && request.URL.Path == "/admin/roles" {
		h.adminRoles(writer, request, "")
		return
	}

	if request.Method == http.MethodGet && request.URL.Path == "/admin/roles/new" {
		h.adminRoleEdit(writer, request, Role{})
		return
	}

	if request.Method == http.MethodPost && request.URL.Path == "/admin/roles" {
		h.adminCreateRole(writer, request)
		return
	}

	if after, ok := strings.CutPrefix(request.URL.Path, "/admin/roles/"); ok {
		roleID := after
		if request.Method == http.MethodGet {
			h.adminRoles(writer, request, roleID)
			return
		}

		if request.Method == http.MethodPost {
			h.adminUpdateRole(writer, request, roleID)
			return
		}
	}

	if request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/admin/users/") {
		h.adminUserPermissions(writer, request)
		return
	}

	if request.Method == http.MethodGet && request.URL.Path == "/admin/audit" {
		h.adminAudit(writer, request)
		return
	}

	writeError(writer, http.StatusNotFound, "not found")
}

func (h Handler) adminPermissions(writer http.ResponseWriter, request *http.Request) {
	snapshot := h.Store.Snapshot()
	tenantID := request.URL.Query().Get("tenant_id")
	selectedProduct := request.URL.Query().Get("product")

	products := permissionProducts(snapshot)
	rows := productPermissionRows(snapshot, tenantID, selectedProduct)

	h.renderAdmin(writer, adminPage{Title: "権限管理", TenantID: tenantID, Products: products, Product: selectedProduct, ProductRows: rows})
}

func permissionProducts(snapshot AuthorizationSnapshot) []string {
	products := make([]string, 0)

	seen := make(map[string]bool)
	for _, permission := range snapshot.Permissions {
		if !seen[permission.Product] {
			products = append(products, permission.Product)
			seen[permission.Product] = true
		}
	}

	sort.Strings(products)

	return products
}

func productPermissionRows(snapshot AuthorizationSnapshot, tenantID, selectedProduct string) []adminProductPermissionRow {
	rows := make([]adminProductPermissionRow, 0)

	for _, permission := range snapshot.Permissions {
		if selectedProduct != "" && permission.Product != selectedProduct {
			continue
		}

		rows = append(rows, adminProductPermissionRow{
			Key: permission.Key, Resource: permission.Resource, Action: permission.Action,
			Licensed: permissionLicensed(snapshot, tenantID, permission.Key), Active: permission.Active,
			Assignable: permission.CustomRoleAssignable, ConfiguredRoles: configuredRoleCount(snapshot, tenantID, permission.Key),
			Requires: strings.Join(permission.Requires, ", "),
		})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })

	return rows
}

func configuredRoleCount(snapshot AuthorizationSnapshot, tenantID, permissionKey string) int {
	count := 0

	for _, role := range snapshot.Roles {
		if role.TenantID != tenantID || !roleHasPermission(role, permissionKey) {
			continue
		}

		count++
	}

	return count
}

func roleHasPermission(role Role, permissionKey string) bool {
	for _, configured := range role.Permissions {
		if configured.Permission == permissionKey && configured.Configured {
			return true
		}
	}

	return false
}

func permissionLicensed(snapshot AuthorizationSnapshot, tenantID, permissionKey string) bool {
	tenant, ok := snapshot.Tenants[tenantID]
	if !ok {
		return false
	}

	for _, entitlement := range tenant.Entitlements {
		if !entitlement.Enabled {
			continue
		}

		if catalog, exists := snapshot.Entitlements[entitlement.Key]; exists && containsPermission(catalog.Permissions, permissionKey) {
			return true
		}
	}

	return false
}

func containsPermission(values []string, target string) bool {
	return slices.Contains(values, target)
}

func (h Handler) adminRoles(writer http.ResponseWriter, request *http.Request, roleID string) {
	snapshot := h.Store.Snapshot()
	tenantID := request.URL.Query().Get("tenant_id")

	if roleID != "" {
		role, ok := snapshot.Roles[roleID]
		if !ok || role.TenantID != tenantID {
			writeError(writer, http.StatusNotFound, "role not found")
			return
		}

		h.adminRoleEdit(writer, request, role)

		return
	}

	roles := make([]Role, 0)

	for _, role := range snapshot.Roles {
		if role.TenantID == tenantID {
			roles = append(roles, role)
		}
	}

	sort.Slice(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })
	h.renderAdmin(writer, adminPage{Title: "ロール管理", TenantID: tenantID, Roles: roles})
}

func (h Handler) adminRoleEdit(writer http.ResponseWriter, request *http.Request, role Role) {
	rows := make([]adminPermissionRow, 0, len(role.Permissions))
	for _, permission := range role.Permissions {
		rows = append(rows, adminPermissionRow{Permission: permission.Permission, Configured: permission.Configured, Scope: permission.Scope})
	}

	h.renderAdmin(writer, adminPage{Title: "ロール編集", TenantID: request.URL.Query().Get("tenant_id"), Role: role, Rows: rows})
}

func (h Handler) adminCreateRole(writer http.ResponseWriter, request *http.Request) {
	if err := request.ParseForm(); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid form")
		return
	}

	role, err := h.Store.CreateRole(formActor(request), Role{ID: request.FormValue("role_id"), TenantID: request.FormValue("tenant_id"), Name: request.FormValue("name"), Permissions: parsePermissionLines(request.FormValue("permissions"))})
	if err != nil {
		h.renderAdmin(writer, adminPage{Title: "ロール作成", TenantID: request.FormValue("tenant_id"), Message: err.Error()})
		return
	}

	h.redirectRole(writer, request, role.ID, role.TenantID)
}

func (h Handler) adminUpdateRole(writer http.ResponseWriter, request *http.Request, roleID string) {
	if err := request.ParseForm(); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid form")
		return
	}

	version, err := strconv.Atoi(request.FormValue("version"))
	if err != nil {
		h.renderAdmin(writer, adminPage{Title: "ロール編集", TenantID: request.FormValue("tenant_id"), Message: "version must be a number"})
		return
	}

	role, err := h.Store.UpdateRole(formActor(request), roleID, RoleUpdate{Name: request.FormValue("name"), Version: version, Permissions: parsePermissionLines(request.FormValue("permissions"))})
	if err != nil {
		h.renderAdmin(writer, adminPage{Title: "ロール編集", TenantID: request.FormValue("tenant_id"), Message: err.Error()})
		return
	}

	h.redirectRole(writer, request, role.ID, role.TenantID)
}

func (h Handler) adminUserPermissions(writer http.ResponseWriter, request *http.Request) {
	userID := strings.TrimPrefix(request.URL.Path, "/admin/users/")
	subject := Subject{Type: SubjectUser, ID: userID, TenantID: request.URL.Query().Get("tenant_id")}
	permissions := h.Store.Authorizer().EffectivePermissions(subject)

	rows := make([]adminPermissionRow, 0, len(permissions))
	for _, permission := range permissions {
		rows = append(rows, adminPermissionRow{Permission: permission.Permission, Effective: permission.Effective, Configured: permission.Configured, Reason: permission.Reason, Scope: permission.Scope, RoleIDs: strings.Join(permission.RoleIDs, ", ")})
	}

	h.renderAdmin(writer, adminPage{Title: "実効権限", TenantID: subject.TenantID, UserID: userID, Rows: rows})
}

func (h Handler) adminAudit(writer http.ResponseWriter, request *http.Request) {
	snapshot := h.Store.Snapshot()
	tenantID := request.URL.Query().Get("tenant_id")
	logs := make([]AuditLog, 0)

	for _, log := range snapshot.AuditLogs {
		if log.TenantID == tenantID {
			logs = append(logs, log)
		}
	}

	h.renderAdmin(writer, adminPage{Title: "監査ログ", TenantID: tenantID, Audit: logs})
}

func formActor(request *http.Request) Subject {
	return Subject{Type: SubjectUser, ID: request.FormValue("actor_id"), TenantID: request.FormValue("tenant_id")}
}

func parsePermissionLines(value string) []RolePermission {
	lines := strings.Split(value, "\n")

	permissions := make([]RolePermission, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), "|", 2)
		if parts[0] == "" {
			continue
		}

		scope := Scope("")
		if len(parts) == 2 {
			scope = Scope(strings.TrimSpace(parts[1]))
		}

		permissions = append(permissions, RolePermission{Permission: parts[0], Scope: scope, Configured: true})
	}

	return permissions
}

func (h Handler) redirectRole(writer http.ResponseWriter, request *http.Request, roleID, tenantID string) {
	http.Redirect(writer, request, "/admin/roles/"+roleID+"?tenant_id="+tenantID, http.StatusSeeOther)
}

func (h Handler) renderAdmin(writer http.ResponseWriter, page adminPage) {
	content := template.Must(template.New("content").Parse(adminContentTemplate))

	full, err := adminTemplate.Clone()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "template error")
		return
	}

	if _, err := full.AddParseTree("content", content.Tree); err != nil {
		writeError(writer, http.StatusInternalServerError, "template error")
		return
	}

	if err := full.Execute(writer, page); err != nil {
		return
	}
}

const adminContentTemplate = `{{define "content"}}
{{if eq .Title "権限管理"}}<p class="muted">プロダクトを選択すると、そのプロダクトのPermissionだけを表示します。契約状況とロール設定状況は対象テナント単位です。</p><p><a class="button" href="/admin/permissions?tenant_id={{.TenantID}}">すべて</a>{{range .Products}} <a class="button" href="/admin/permissions?tenant_id={{$.TenantID}}&amp;product={{.}}">{{if eq . $.Product}}選択中: {{end}}{{if .}}{{.}}{{else}}その他{{end}}</a>{{end}}</p>
<table><tr><th>Permission</th><th>Resource / Action</th><th>契約</th><th>状態</th><th>カスタムロール</th><th>設定ロール数</th><th>依存</th></tr>{{range .ProductRows}}<tr><td><code>{{.Key}}</code></td><td>{{.Resource}} / {{.Action}}</td><td class="{{if .Licensed}}ok{{else}}bad{{end}}">{{if .Licensed}}契約中{{else}}未契約{{end}}</td><td>{{if .Active}}有効{{else}}無効{{end}}</td><td>{{if .Assignable}}可{{else}}不可{{end}}</td><td>{{.ConfiguredRoles}}</td><td>{{if .Requires}}<code>{{.Requires}}</code>{{else}}—{{end}}</td></tr>{{else}}<tr><td colspan="7" class="muted">Permissionがありません</td></tr>{{end}}</table>
{{else if eq .Title "ロール管理"}}<p><a class="button" href="/admin/roles/new?tenant_id={{.TenantID}}">新しいロール</a></p>
<table><tr><th>名前</th><th>種別</th><th>状態</th><th>Version</th></tr>{{range .Roles}}<tr><td><a href="/admin/roles/{{.ID}}?tenant_id={{.TenantID}}">{{.Name}}</a></td><td>{{if .System}}system{{else}}custom{{end}}</td><td>{{if .Active}}有効{{else}}無効{{end}}</td><td>{{.Version}}</td></tr>{{else}}<tr><td colspan="4" class="muted">ロールがありません</td></tr>{{end}}</table>
{{else if eq .Title "ロール編集"}}<form method="post" action="{{if .Role.ID}}/admin/roles/{{.Role.ID}}{{else}}/admin/roles{{end}}"><label>テナントID</label><input name="tenant_id" value="{{.TenantID}}" required><label>操作者ユーザーID</label><input name="actor_id" required><label>ロールID</label><input name="role_id" value="{{.Role.ID}}" {{if .Role.ID}}readonly{{end}} required><label>名前</label><input name="name" value="{{.Role.Name}}" required><input type="hidden" name="version" value="{{.Role.Version}}"><label>Permission（1行1件、必要なら <code>permission|scope</code>）</label><textarea name="permissions">{{range .Rows}}{{.Permission}}{{if .Scope}}|{{.Scope}}{{end}}
{{end}}</textarea><p class="muted">保存時にPermissionの存在、依存関係、委任範囲、楽観ロックを検証します。</p><button type="submit">保存</button></form>
{{else if eq .Title "実効権限"}}<p>ユーザー: <code>{{.UserID}}</code></p><table><tr><th>Permission</th><th>Configured</th><th>Effective</th><th>Scope</th><th>Reason</th><th>Role</th></tr>{{range .Rows}}<tr><td>{{.Permission}}</td><td>{{.Configured}}</td><td class="{{if .Effective}}ok{{else}}bad{{end}}">{{.Effective}}</td><td>{{.Scope}}</td><td>{{.Reason}}</td><td>{{.RoleIDs}}</td></tr>{{else}}<tr><td colspan="6" class="muted">権限がありません</td></tr>{{end}}</table>
{{else}}<table><tr><th>時刻</th><th>Action</th><th>Actor</th><th>Target</th><th>Result</th><th>Reason</th></tr>{{range .Audit}}<tr><td>{{.CreatedAt}}</td><td>{{.Action}}</td><td>{{.ActorID}}</td><td>{{.TargetType}}/{{.TargetID}}</td><td>{{.Result}}</td><td>{{.Reason}}</td></tr>{{else}}<tr><td colspan="6" class="muted">監査ログがありません</td></tr>{{end}}</table>{{end}}
{{end}}`
