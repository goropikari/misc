# 認可設定 YAML 設計

## 1. ディレクトリ構成

```text
authorization/
├── catalog/
│   ├── products.yaml
│   ├── permissions/
│   │   ├── billing.yaml
│   │   ├── analytics.yaml
│   │   └── organization.yaml
│   ├── entitlements/
│   │   ├── billing.yaml
│   │   └── analytics.yaml
│   ├── system-roles/
│   │   ├── billing.yaml
│   │   └── organization.yaml
│   └── bundles/
│       └── billing.yaml
│
├── routes/
│   ├── billing.yaml
│   └── analytics.yaml
│
├── services/
│   └── service-permissions.yaml
│
└── envoy/
    └── envoy.yaml
```

責務は以下のように分ける。

| ファイル                   | 内容                       |
| -------------------------- | -------------------------- |
| `products.yaml`            | プロダクト定義             |
| `permissions/*.yaml`       | Permissionマスタ           |
| `entitlements/*.yaml`      | 契約機能とPermissionの対応 |
| `system-roles/*.yaml`      | 固定ロール                 |
| `bundles/*.yaml`           | UI上のPermissionグループ   |
| `routes/*.yaml`            | APIルートと必要権限の対応  |
| `service-permissions.yaml` | サービス間認可             |
| `envoy.yaml`               | ext_authzフィルター設定    |

---

# 2. 共通ルール

すべての設定ファイルに、次のトップレベル項目を持たせる。

```yaml
apiVersion: authorization.company.example/v1
kind: PermissionCatalog

metadata:
  product: billing
  owner: team-billing-platform
  description: BillingプロダクトのPermission定義
```

`apiVersion` はスキーマの互換性管理に使用する。

`kind` はファイルの種類を表す。

使用する `kind` は以下とする。

```text
ProductCatalog
PermissionCatalog
EntitlementCatalog
SystemRoleCatalog
PermissionBundleCatalog
RouteAuthorizationCatalog
ServiceAuthorizationCatalog
```

---

# 3. プロダクト定義

`catalog/products.yaml`

```yaml
apiVersion: authorization.company.example/v1
kind: ProductCatalog

metadata:
  owner: team-authorization-platform
  description: 全社プロダクトカタログ

products:
  - key: billing
    displayName: Billing
    description: 請求書・支払い・承認管理
    owner: team-billing-platform
    status: active

  - key: analytics
    displayName: Analytics
    description: レポート・分析・データ出力
    owner: team-analytics
    status: active

  - key: organization
    displayName: Organization
    description: 組織・ユーザー・ロール管理
    owner: team-identity-platform
    status: active
```

`product.key` は、PermissionやEntitlementの名前空間として使用する。

---

# 4. Permission定義

`catalog/permissions/billing.yaml`

```yaml
apiVersion: authorization.company.example/v1
kind: PermissionCatalog

metadata:
  product: billing
  owner: team-billing-platform
  description: BillingプロダクトのPermission

permissions:
  - key: billing.invoice.read
    displayName: 請求書を閲覧
    description: 許可された範囲の請求書を閲覧できる

    resource: invoice
    action: read

    riskLevel: low
    status: active

    assignment:
      customRoleAssignable: true
      delegatable: true

    scope:
      type: data-range
      allowedValues:
        - own
        - department
        - organization
      defaultValue: own

    dependencies: []

    ui:
      group: invoice
      order: 100

  - key: billing.invoice.create
    displayName: 請求書を作成
    description: 新しい請求書を作成できる

    resource: invoice
    action: create

    riskLevel: medium
    status: active

    assignment:
      customRoleAssignable: true
      delegatable: true

    scope:
      type: none

    dependencies:
      - billing.invoice.read

    ui:
      group: invoice
      order: 110

  - key: billing.invoice.update
    displayName: 請求書を編集
    description: 許可された範囲の請求書を編集できる

    resource: invoice
    action: update

    riskLevel: medium
    status: active

    assignment:
      customRoleAssignable: true
      delegatable: true

    scope:
      type: data-range
      allowedValues:
        - own
        - department
        - organization
      defaultValue: own

    dependencies:
      - billing.invoice.read

    ui:
      group: invoice
      order: 120

  - key: billing.invoice.delete
    displayName: 請求書を削除
    description: 許可された範囲の請求書を削除できる

    resource: invoice
    action: delete

    riskLevel: high
    status: active

    assignment:
      customRoleAssignable: true
      delegatable: true

    scope:
      type: data-range
      allowedValues:
        - own
        - department
        - organization
      defaultValue: own

    dependencies:
      - billing.invoice.read

    ui:
      group: invoice
      order: 130
      confirmationRequired: true

  - key: billing.invoice.submit
    displayName: 請求書を承認申請
    description: 請求書を承認ワークフローへ申請できる

    resource: invoice
    action: submit

    riskLevel: medium
    status: active

    assignment:
      customRoleAssignable: true
      delegatable: true

    scope:
      type: data-range
      allowedValues:
        - own
        - department
      defaultValue: own

    dependencies:
      - billing.invoice.read

    ui:
      group: approval
      order: 200

  - key: billing.invoice.approve
    displayName: 請求書を承認
    description: 申請された請求書を承認できる

    resource: invoice
    action: approve

    riskLevel: high
    status: active

    assignment:
      customRoleAssignable: true
      delegatable: true

    scope:
      type: data-range
      allowedValues:
        - department
        - organization
      defaultValue: department

    dependencies:
      - billing.invoice.read

    ui:
      group: approval
      order: 210
      warning: 承認操作は監査ログに記録されます

  - key: billing.approval-rule.read
    displayName: 承認ルールを閲覧
    description: 承認ワークフローの設定を閲覧できる

    resource: approval-rule
    action: read

    riskLevel: medium
    status: active

    assignment:
      customRoleAssignable: true
      delegatable: true

    scope:
      type: none

    dependencies: []

    ui:
      group: approval-rule
      order: 300

  - key: billing.approval-rule.update
    displayName: 承認ルールを編集
    description: 承認ワークフローの設定を変更できる

    resource: approval-rule
    action: update

    riskLevel: critical
    status: active

    assignment:
      customRoleAssignable: true
      delegatable: restricted

    scope:
      type: none

    dependencies:
      - billing.approval-rule.read

    ui:
      group: approval-rule
      order: 310
      warning: 組織全体の承認フローに影響します
      confirmationRequired: true
```

## Permissionキーの命名規則

30プロダクトで衝突を避けるため、次の形式を推奨する。

```text
product.resource.action
```

例:

```text
billing.invoice.read
billing.invoice.approve
analytics.report.export
organization.role.update
```

プロダクト内だけで完全に閉じる場合は `invoice.read` でもよいが、共通認可基盤ではプロダクトプレフィックスを付けたほうが安全である。

---

# 5. UIグループ定義

Permissionの `ui.group` が参照する表示グループを定義する。

`catalog/permissions/billing.yaml` に含めても、別ファイルに分けてもよい。

```yaml
uiGroups:
  - key: invoice
    displayName: 請求書
    description: 請求書の閲覧・作成・更新・削除
    order: 100

  - key: approval
    displayName: 請求書の承認
    description: 承認申請と承認操作
    order: 200

  - key: approval-rule
    displayName: 承認ルール
    description: 承認ワークフローの設定
    order: 300
```

権限管理画面では次のように表示する。

```text
Billing

  請求書
    □ 請求書を閲覧
    □ 請求書を作成
    □ 請求書を編集
    □ 請求書を削除

  請求書の承認
    □ 請求書を承認申請
    □ 請求書を承認

  承認ルール
    □ 承認ルールを閲覧
    □ 承認ルールを編集
```

---

# 6. Entitlement定義

`catalog/entitlements/billing.yaml`

```yaml
apiVersion: authorization.company.example/v1
kind: EntitlementCatalog

metadata:
  product: billing
  owner: team-billing-commercial-platform
  description: Billingの契約機能

entitlements:
  - key: billing.core
    displayName: Billing Core
    description: Billingの基本機能
    status: active

    grants:
      permissions:
        - billing.invoice.read
        - billing.invoice.create
        - billing.invoice.update
        - billing.invoice.delete

  - key: billing.approval
    displayName: 承認ワークフロー
    description: 請求書の申請・承認と承認ルール管理
    status: active

    requires:
      entitlements:
        - billing.core

    grants:
      permissions:
        - billing.invoice.submit
        - billing.invoice.approve
        - billing.approval-rule.read
        - billing.approval-rule.update

  - key: billing.approval.read-only
    displayName: 承認ワークフロー閲覧
    description: 承認状態と承認ルールの閲覧のみ
    status: active

    requires:
      entitlements:
        - billing.core

    grants:
      permissions:
        - billing.approval-rule.read
```

ここでの `grants.permissions` は、Permissionをユーザーへ直接付与する意味ではない。

意味は次のとおり。

```text
このEntitlementを持つテナントでは、
指定されたPermissionをロールへ設定でき、
認可判定の対象として有効化できる
```

実効権限は以下で計算する。

```text
effective permissions
=
role permissions
∩
tenant entitlement permissions
```

---

# 7. Entitlementの除外設定

一部の契約で特定Permissionだけ利用不可にする必要がある場合は、例外設定を持たせることもできる。

```yaml
entitlements:
  - key: billing.approval.limited
    displayName: 承認ワークフロー Limited

    grants:
      permissions:
        - billing.invoice.submit
        - billing.invoice.approve
        - billing.approval-rule.read

    excludes:
      permissions:
        - billing.approval-rule.update
```

ただし、`grants` と `excludes` の組み合わせが増えると理解が難しくなる。

可能なら、最終的に利用可能なPermissionを `grants.permissions` に明示し、`excludes` は使わないほうがよい。

---

# 8. システムロール定義

`catalog/system-roles/billing.yaml`

```yaml
apiVersion: authorization.company.example/v1
kind: SystemRoleCatalog

metadata:
  product: billing
  owner: team-billing-platform
  description: Billingのシステムロール

roles:
  - key: billing.viewer
    displayName: Billing閲覧者
    description: 請求書と承認設定を閲覧できる
    status: active

    mutable: false
    assignableByTenantAdmin: true

    permissions:
      - key: billing.invoice.read
        scope: organization

      - key: billing.approval-rule.read

  - key: billing.operator
    displayName: Billing担当者
    description: 請求書の作成・編集・申請ができる
    status: active

    mutable: false
    assignableByTenantAdmin: true

    permissions:
      - key: billing.invoice.read
        scope: department

      - key: billing.invoice.create

      - key: billing.invoice.update
        scope: department

      - key: billing.invoice.submit
        scope: own

  - key: billing.approver
    displayName: 請求書承認者
    description: 所属部署の請求書を承認できる
    status: active

    mutable: false
    assignableByTenantAdmin: true

    permissions:
      - key: billing.invoice.read
        scope: department

      - key: billing.invoice.approve
        scope: department

  - key: billing.admin
    displayName: Billing管理者
    description: Billingのすべての管理操作を実行できる
    status: active

    mutable: false
    assignableByTenantAdmin: restricted

    permissions:
      - key: billing.invoice.read
        scope: organization

      - key: billing.invoice.create

      - key: billing.invoice.update
        scope: organization

      - key: billing.invoice.delete
        scope: organization

      - key: billing.invoice.submit
        scope: department

      - key: billing.invoice.approve
        scope: organization

      - key: billing.approval-rule.read

      - key: billing.approval-rule.update
```

カスタムロールはこのファイルでは管理しない。

カスタムロールは、ユーザー操作によってDBに保存する。

---

# 9. Permissionバンドル

内部PermissionをUI上でまとめたい場合だけ使用する。

`catalog/bundles/billing.yaml`

```yaml
apiVersion: authorization.company.example/v1
kind: PermissionBundleCatalog

metadata:
  product: billing
  owner: team-billing-platform
  description: Billing権限管理画面のPermissionバンドル

bundles:
  - key: billing.invoice.basic-management
    displayName: 請求書の基本管理
    description: 請求書の閲覧・作成・編集をまとめて許可する

    permissions:
      - key: billing.invoice.read
        scopeBehavior: inherit

      - key: billing.invoice.create

      - key: billing.invoice.update
        scopeBehavior: inherit

    ui:
      order: 100
      scopeSelector:
        enabled: true
        appliesTo:
          - billing.invoice.read
          - billing.invoice.update

  - key: billing.approval.viewer
    displayName: 承認設定の閲覧
    description: 承認ルール関連の閲覧権限

    permissions:
      - key: billing.approval-rule.read

    ui:
      order: 200
```

バンドルへ以下のような高権限操作を安易に含めない。

```text
delete
approve
refund
export
role.update
approval-rule.update
```

保存時には、バンドルを展開したPermissionを `RolePermission` として保存する。

---

# 10. APIルート認可定義

`routes/billing.yaml`

```yaml
apiVersion: authorization.company.example/v1
kind: RouteAuthorizationCatalog

metadata:
  product: billing
  owner: team-billing-api
  description: Billing APIの認可要件

defaults:
  authenticationRequired: true
  tenantRequired: true
  tenantMembershipRequired: true

routes:
  - id: billing.invoice.list
    method: GET
    pathTemplate: /v1/invoices

    authorization:
      entitlement: billing.core
      permission: billing.invoice.read

    enforcement:
      platform: ext-authz
      domain: application

    domainChecks:
      - result-row-filtering
      - tenant-boundary
      - scope-filtering

  - id: billing.invoice.get
    method: GET
    pathTemplate: /v1/invoices/{invoiceId}

    authorization:
      entitlement: billing.core
      permission: billing.invoice.read

    enforcement:
      platform: ext-authz
      domain: application

    resource:
      type: invoice
      idFrom:
        pathParameter: invoiceId

    domainChecks:
      - resource-tenant
      - resource-scope

  - id: billing.invoice.create
    method: POST
    pathTemplate: /v1/invoices

    authorization:
      entitlement: billing.core
      permission: billing.invoice.create

    enforcement:
      platform: ext-authz
      domain: application

    domainChecks:
      - destination-department
      - business-validation

  - id: billing.invoice.update
    method: PATCH
    pathTemplate: /v1/invoices/{invoiceId}

    authorization:
      entitlement: billing.core
      permission: billing.invoice.update

    enforcement:
      platform: ext-authz
      domain: application

    resource:
      type: invoice
      idFrom:
        pathParameter: invoiceId

    domainChecks:
      - resource-tenant
      - resource-scope
      - invoice-editable-status

  - id: billing.invoice.delete
    method: DELETE
    pathTemplate: /v1/invoices/{invoiceId}

    authorization:
      entitlement: billing.core
      permission: billing.invoice.delete

    enforcement:
      platform: ext-authz
      domain: application

    resource:
      type: invoice
      idFrom:
        pathParameter: invoiceId

    domainChecks:
      - resource-tenant
      - resource-scope
      - invoice-deletable-status

  - id: billing.invoice.submit
    method: POST
    pathTemplate: /v1/invoices/{invoiceId}/submit

    authorization:
      entitlement: billing.approval
      permission: billing.invoice.submit

    enforcement:
      platform: ext-authz
      domain: application

    resource:
      type: invoice
      idFrom:
        pathParameter: invoiceId

    domainChecks:
      - resource-tenant
      - resource-scope
      - invoice-submittable-status

  - id: billing.invoice.approve
    method: POST
    pathTemplate: /v1/invoices/{invoiceId}/approve

    authorization:
      entitlement: billing.approval
      permission: billing.invoice.approve

    enforcement:
      platform: ext-authz
      domain: application

    resource:
      type: invoice
      idFrom:
        pathParameter: invoiceId

    domainChecks:
      - resource-tenant
      - resource-scope
      - invoice-approvable-status
      - approval-limit
      - self-approval-prohibited

  - id: billing.health
    method: GET
    pathTemplate: /health

    authorization:
      mode: public
```

`domainChecks` はext_authzが実行する共通ポリシーではない。

各APIがアプリケーション内部で実装すべきチェックを明示し、設計レビューやテスト生成に使う。

---

# 11. 1ルートに複数Permissionが必要な場合

通常は1つのAPIに1つの主要Permissionを割り当てる。

複数必要な場合は、条件を明示する。

```yaml
routes:
  - id: billing.invoice.bulk-export
    method: POST
    pathTemplate: /v1/invoices/bulk-export

    authorization:
      entitlement: billing.export

      permissions:
        allOf:
          - billing.invoice.read
          - billing.invoice.export
```

いずれか1つでよい場合:

```yaml
authorization:
  permissions:
    anyOf:
      - billing.invoice.approve
      - billing.invoice.override-approval
```

ただし、`allOf` や `anyOf` を多用すると権限体系が理解しにくくなる。

可能なら、API操作に対応した専用Permissionを作る。

```text
billing.invoice.bulk-export
```

---

# 12. サービス間認可

`services/service-permissions.yaml`

```yaml
apiVersion: authorization.company.example/v1
kind: ServiceAuthorizationCatalog

metadata:
  owner: team-authorization-platform
  description: サービス間アクセス制御

servicePermissions:
  - key: billing.invoice.read-summary-internal
    displayName: 請求書サマリー内部参照
    description: 分析用途の請求書サマリー参照

    resource: invoice-summary
    action: read
    audience: service

    callers:
      - principal: spiffe://company.example/ns/analytics/sa/analytics-api
      - principal: spiffe://company.example/ns/reporting/sa/report-worker

    destinations:
      - service: billing-api
        routes:
          - billing.internal.invoice-summary.list

  - key: organization.user.read-internal
    displayName: ユーザー内部参照

    resource: organization-user
    action: read
    audience: service

    callers:
      - principal: spiffe://company.example/ns/billing/sa/billing-api

    destinations:
      - service: organization-api
        routes:
          - organization.internal.user.get
```

ユーザー向けPermissionとサービス向けPermissionを分ける。

```text
billing.invoice.read
billing.invoice.read-summary-internal
```

---

# 13. Envoy設定例

`envoy/envoy.yaml`

以下は概念的な最小例。

```yaml
static_resources:
  listeners:
    - name: public_api_listener

      address:
        socket_address:
          address: 0.0.0.0
          port_value: 8080

      filter_chains:
        - filters:
            - name: envoy.filters.network.http_connection_manager

              typed_config:
                "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager

                stat_prefix: public_api
                route_config:
                  name: api_routes

                  virtual_hosts:
                    - name: billing_api
                      domains:
                        - "*"

                      routes:
                        - match:
                            path: /health

                          route:
                            cluster: billing_api

                          typed_per_filter_config:
                            envoy.filters.http.ext_authz:
                              "@type": type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthzPerRoute
                              disabled: true

                        - match:
                            safe_regex:
                              regex: "^/v1/invoices/[^/]+/approve$"

                          route:
                            cluster: billing_api

                          metadata:
                            filter_metadata:
                              company.authorization:
                                route_id: billing.invoice.approve
                                product: billing
                                resource: invoice
                                action: approve
                                entitlement: billing.approval
                                permission: billing.invoice.approve

                        - match:
                            prefix: /v1/invoices

                          route:
                            cluster: billing_api

                http_filters:
                  - name: envoy.filters.http.jwt_authn
                    typed_config:
                      "@type": type.googleapis.com/envoy.extensions.filters.http.jwt_authn.v3.JwtAuthentication
                      # JWT検証設定

                  - name: envoy.filters.http.ext_authz

                    typed_config:
                      "@type": type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz

                      grpc_service:
                        envoy_grpc:
                          cluster_name: authorization_service

                        timeout: 0.2s

                      transport_api_version: V3

                      failure_mode_allow: false

                      metadata_context_namespaces:
                        - company.authorization

                  - name: envoy.filters.http.router
                    typed_config:
                      "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router

  clusters:
    - name: authorization_service

      type: STRICT_DNS

      load_assignment:
        cluster_name: authorization_service
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: authorization-service
                      port_value: 9001

      typed_extension_protocol_options:
        envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
          "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
          explicit_http_config:
            http2_protocol_options: {}

    - name: billing_api

      type: STRICT_DNS

      load_assignment:
        cluster_name: billing_api
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: billing-api
                      port_value: 8080
```

ルートメタデータには、最低限次を入れる。

```yaml
company.authorization:
  route_id: billing.invoice.approve
  product: billing
  resource: invoice
  action: approve
  entitlement: billing.approval
  permission: billing.invoice.approve
```

認可サービスは、URLからPermissionを推測せず、このメタデータを認可要求として使用する。

---

# 14. 認可サービスが受け取る正規化データ

EnvoyのCheckRequestから、認可サービス内部では次の形式へ正規化する。

```yaml
request:
  requestId: req-01JXYZ
  routeId: billing.invoice.approve

subject:
  type: user
  id: user-123
  tenantId: tenant-456
  sessionId: session-789

authentication:
  method: oidc
  assuranceLevel: 2

target:
  product: billing
  resource: invoice
  action: approve
  resourceId: invoice-999

requirements:
  entitlement: billing.approval
  permission: billing.invoice.approve

http:
  method: POST
  path: /v1/invoices/invoice-999/approve

context:
  sourceIp: 192.0.2.10
  userAgent: example-client
```

認可サービスの判定:

```text
1. Subjectが有効か
2. Tenantが有効か
3. SubjectがTenantに所属しているか
4. TenantがEntitlementを持つか
5. SubjectのRoleがPermissionを持つか
6. Roleが有効か
7. 単純なScope条件を満たすか
```

---

# 15. カスタムロール作成APIの入力例

カスタムロールは設定ファイルではなくDBへ保存する。

```yaml
name: 経理担当
description: 所属部署の請求書を作成・編集する担当者

permissions:
  - key: billing.invoice.read
    scope: department

  - key: billing.invoice.create

  - key: billing.invoice.update
    scope: department

  - key: billing.invoice.submit
    scope: own
```

保存後のDB上の概念:

```yaml
role:
  id: role-custom-123
  tenantId: tenant-456
  type: custom
  name: 経理担当
  version: 3
  status: active

rolePermissions:
  - permission: billing.invoice.read
    scope: department

  - permission: billing.invoice.create

  - permission: billing.invoice.update
    scope: department

  - permission: billing.invoice.submit
    scope: own
```

---

# 16. 設定済みだが未契約のPermission

契約が失効しても、ロール設定は削除しない。

APIレスポンス例:

```yaml
rolePermission:
  permission: billing.invoice.approve
  scope: department

  configured: true
  effective: false

  ineffectiveReasons:
    - code: ENTITLEMENT_MISSING
      entitlement: billing.approval
```

権限管理画面では次のように表示する。

```text
☑ 請求書を承認
  現在は「承認ワークフロー」が未契約のため無効です
```

---

# 17. 委任制御設定

Permissionマスタの `delegatable` だけでは足りない場合、委任ポリシーを別ファイルにする。

```yaml
apiVersion: authorization.company.example/v1
kind: DelegationPolicyCatalog

metadata:
  product: organization
  owner: team-identity-platform

policies:
  - key: tenant-role-admin-default

    appliesTo:
      actorPermission: organization.role.update

    allow:
      riskLevels:
        - low
        - medium
        - high

    deny:
      permissions:
        - organization.super-admin.assign
        - organization.tenant.delete
        - billing.approval-rule.update

  - key: security-admin

    appliesTo:
      actorPermission: organization.security-role.update

    allow:
      permissions:
        - billing.approval-rule.update
        - analytics.audit-log.read
```

ロール編集時の付与可能条件:

```text
customRoleAssignable = true
AND delegatable != false
AND tenant entitlement is active
AND delegation policy allows
```

---

# 18. 静的検証ルール

CIで、すべてのYAMLに対して以下を検証する。

```yaml
validationRules:
  permissionKeys:
    pattern: "^[a-z][a-z0-9-]*\\.[a-z][a-z0-9-]*\\.[a-z][a-z0-9-]*$"
    unique: true

  entitlementKeys:
    pattern: "^[a-z][a-z0-9-]*\\.[a-z][a-z0-9.-]*$"
    unique: true

  references:
    entitlementPermissionsMustExist: true
    rolePermissionsMustExist: true
    routePermissionsMustExist: true
    dependenciesMustExist: true

  dependencies:
    rejectCycles: true

  scope:
    roleScopeMustBeAllowedByPermission: true
    defaultScopeMustBeAllowed: true

  routes:
    uniqueRouteId: true
    protectedRoutesRequirePermission: true
    protectedRoutesRequireEntitlement: true

  security:
    criticalPermissionsRequireWarning: true
    criticalPermissionsRequireExplicitDelegationPolicy: true
```

検証エラー例:

```text
ERROR:
routes/billing.yaml

route billing.invoice.approve references unknown permission:
billing.invoice.approval
```

```text
ERROR:
catalog/permissions/billing.yaml

dependency cycle detected:
billing.invoice.read
→ billing.invoice.update
→ billing.invoice.read
```

```text
ERROR:
catalog/system-roles/billing.yaml

scope "organization" is not allowed for:
billing.invoice.submit

allowed:
- own
- department
```

---

# 19. 推奨する初期構成

初期リリースでは、以下の構成で十分。

```text
PermissionCatalog
EntitlementCatalog
SystemRoleCatalog
RouteAuthorizationCatalog
ServiceAuthorizationCatalog
```

最初から導入しなくてもよいもの:

```text
PermissionBundleCatalog
DelegationPolicyCatalog
CapabilityCatalog
汎用Policy DSL
```

最初の関係はシンプルに保つ。

```text
Entitlement
  └─ Permissions

System Role
  └─ Permissions

Custom Role
  └─ Permissions

API Route
  ├─ Required Entitlement
  └─ Required Permission
```

実効権限:

```text
effective permission
=
permission configured in active user roles
AND
permission enabled by active tenant entitlements
```

---

# 20. 最小構成の具体例

小さく始める場合、Billingだけなら以下の3ファイルでもよい。

## `permissions.yaml`

```yaml
apiVersion: authorization.company.example/v1
kind: PermissionCatalog

metadata:
  product: billing
  owner: team-billing

permissions:
  - key: billing.invoice.read
    displayName: 請求書を閲覧
    resource: invoice
    action: read
    riskLevel: low

    assignment:
      customRoleAssignable: true
      delegatable: true

    scope:
      type: data-range
      allowedValues:
        - own
        - department
        - organization
      defaultValue: own

  - key: billing.invoice.approve
    displayName: 請求書を承認
    resource: invoice
    action: approve
    riskLevel: high

    assignment:
      customRoleAssignable: true
      delegatable: true

    scope:
      type: data-range
      allowedValues:
        - department
        - organization
      defaultValue: department

    dependencies:
      - billing.invoice.read
```

## `entitlements.yaml`

```yaml
apiVersion: authorization.company.example/v1
kind: EntitlementCatalog

metadata:
  product: billing
  owner: team-billing

entitlements:
  - key: billing.core
    displayName: Billing Core
    grants:
      permissions:
        - billing.invoice.read

  - key: billing.approval
    displayName: 承認ワークフロー
    requires:
      entitlements:
        - billing.core
    grants:
      permissions:
        - billing.invoice.approve
```

## `routes.yaml`

```yaml
apiVersion: authorization.company.example/v1
kind: RouteAuthorizationCatalog

metadata:
  product: billing
  owner: team-billing

routes:
  - id: billing.invoice.get
    method: GET
    pathTemplate: /v1/invoices/{invoiceId}

    authorization:
      entitlement: billing.core
      permission: billing.invoice.read

    domainChecks:
      - resource-tenant
      - resource-scope

  - id: billing.invoice.approve
    method: POST
    pathTemplate: /v1/invoices/{invoiceId}/approve

    authorization:
      entitlement: billing.approval
      permission: billing.invoice.approve

    domainChecks:
      - resource-tenant
      - resource-scope
      - invoice-approvable-status
      - approval-limit
      - self-approval-prohibited
```
