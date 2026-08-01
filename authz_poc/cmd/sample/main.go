package main

import (
	"fmt"

	"github.com/goropikari/go-project/authz"
)

func main() {
	authorizer := authz.Authorizer{
		Tenants:      map[string]authz.Tenant{"tenant-1": {ID: "tenant-1", Active: true, Members: map[string]bool{"user-1": true}, Entitlements: []authz.TenantEntitlement{{Key: "billing.approval", Enabled: false}}}},
		Entitlements: map[string]authz.Entitlement{"billing.approval": {Key: "billing.approval", Permissions: []string{"invoice.approve"}}},
		Roles:        map[string]authz.Role{"role-1": {ID: "role-1", TenantID: "tenant-1", Name: "請求承認者", Active: true, Permissions: []authz.RolePermission{{Permission: "invoice.approve", Configured: true}}}},
		UserRoles:    []authz.UserRole{{UserID: "user-1", RoleID: "role-1", Valid: true}},
	}
	request := authz.AuthorizationRequest{Subject: authz.Subject{Type: authz.SubjectUser, ID: "user-1", TenantID: "tenant-1"}, Entitlement: "billing.approval", Permission: "invoice.approve"}
	decision := authorizer.Evaluate(request)
	fmt.Printf("allowed=%t reason=%s\n", decision.Allowed, decision.Reason)

	for _, permission := range authorizer.EffectivePermissions(request.Subject) {
		fmt.Printf("permission=%s configured=%t effective=%t reason=%s\n", permission.Permission, permission.Configured, permission.Effective, permission.Reason)
	}
}
