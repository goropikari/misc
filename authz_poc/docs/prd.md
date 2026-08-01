# 共通認可基盤 設計案

## 1. 目的

約30のプロダクトで、以下を一貫して扱える共通認可基盤を構築する。

- テナントの契約・ライセンス制御
- ユーザーのロール管理
- ユーザー作成のカスタムロール
- API単位のPermission認可
- データ範囲の制御
- サービス間認可
- 権限変更の即時反映
- 権限管理画面
- 監査ログ

API入口の共通認可には Envoy `ext_authz` を使用する。

---

# 2. 基本概念

## 2.1 Entitlement

テナントが契約上利用できる機能を表す。

例:

```text
billing.basic
billing.approval
report.export
ai.assistant
```

Entitlementはテナント単位で付与される。

```text
Tenant
  └─ Entitlements
```

Entitlementは「会社として利用できるか」を表し、個々のユーザーが利用できるかは表さない。

---

## 2.2 Permission

ユーザーまたはサービスが実行できる操作を表す。

命名形式は原則として次を使用する。

```text
resource.action
```

例:

```text
invoice.read
invoice.create
invoice.update
invoice.delete
invoice.submit
invoice.approve

approval_rule.read
approval_rule.update

report.read
report.export

role.read
role.create
role.update
role.delete
role.assign
```

CRUDで表現しにくい業務操作は、専用の動詞を使用する。

```text
invoice.approve
payment.refund
order.cancel
report.export
```

---

## 2.3 Role

Permissionの集合を表す。

```text
Role
  └─ Permissions
```

Roleには次の2種類がある。

### システムロール

プロダクト側が定義する固定ロール。

例:

```text
Billing Admin
Billing Viewer
Organization Admin
```

原則として、テナント管理者は削除・定義変更できない。

### カスタムロール

テナント管理者が作成する動的ロール。

例:

```text
経理担当
請求書承認者
監査閲覧者
```

テナント管理者は、許可された範囲内でPermissionを選択してロールを作成する。

---

## 2.4 Scope

Permissionが適用されるデータ範囲を表す。

例:

```text
own
team
department
organization
all
```

すべてのPermissionにScopeを付ける必要はない。

データ範囲が意味を持つPermissionだけに付与する。

例:

```json
{
  "permission": "invoice.read",
  "scope": "department"
}
```

Scopeが排他的な場合は、複数Permissionとして表現せず、1つのPermissionに対する属性として扱う。

---

## 2.5 Policy

リソース属性やユーザー属性を使った条件判定を表す。

例:

```text
invoice.owner_id == user.id
invoice.department_id == user.department_id
invoice.tenant_id == user.tenant_id
invoice.amount <= user.approval_limit
```

Permissionが「何をしてよいか」を表すのに対し、Policyは「どの対象に対して可能か」を表す。

---

# 3. EntitlementとPermissionの関係

Entitlementには、利用可能になるPermissionを紐づける。

例:

```yaml
entitlement: billing.approval

permissions:
  - invoice.submit
  - invoice.approve
  - approval_rule.read
  - approval_rule.update
```

意味は以下のとおり。

```text
billing.approvalを契約しているテナントでは、
これらのPermissionを利用対象にできる
```

EntitlementがPermissionを直接ユーザーへ付与するわけではない。

最終的な利用可否は、EntitlementとRoleの両方で判断する。

```text
実効Permission
=
Roleに設定されたPermission
∩
Entitlementで利用可能なPermission
```

例:

```text
テナントのEntitlementで利用可能:
- invoice.read
- invoice.update
- invoice.approve

ユーザーのRole:
- invoice.read
- invoice.approve
- report.export

実効Permission:
- invoice.read
- invoice.approve
```

---

# 4. ライセンス変更時の扱い

Entitlementが失効しても、カスタムロールからPermissionを削除しない。

例:

```text
カスタムロール設定:
- invoice.read
- report.export
```

`report.export` のEntitlementが失効した場合:

```text
invoice.read
  configured: true
  effective: true

report.export
  configured: true
  effective: false
  reason: ENTITLEMENT_MISSING
```

これにより、再契約したときにロール設定を復元できる。

ただし、機密性の高いPermissionについては、再契約後に再付与を要求する運用も選択可能とする。

---

# 5. ユーザーとRoleの関係

ユーザーにはPermissionを直接付与せず、原則としてRoleを付与する。

```text
User
  └─ UserRole
       └─ Role
            └─ RolePermission
```

複数Roleを持つ場合、Permissionは和集合とする。

```text
user_role_permissions
=
role_a.permissions
∪
role_b.permissions
```

その後、Entitlementとの積集合を取る。

```text
effective_permissions
=
user_role_permissions
∩
licensed_permissions
```

マルチテナントの場合、Role割り当てはテナント単位で保持する。

```text
UserRole
- user_id
- tenant_id
- role_id
```

同じユーザーが、テナントごとに異なるRoleを持てるようにする。

---

# 6. 委任制御

ロール管理者が、任意のPermissionを他者へ付与できないようにする。

Permissionには委任可能性を持たせる。

```yaml
permission: invoice.approve
delegatable: true
```

または、操作者ごとに委任可能Permissionを保持する。

```text
DelegatablePermission
- subject_id
- tenant_id
- permission_id
- max_scope
```

ロールへPermissionを追加できる条件は次のとおり。

```text
付与可能
=
テナントのEntitlementで利用可能
AND
操作者が委任可能
AND
対象Permissionがカスタムロールで利用可能
```

`role.manage` を持つだけで、すべてのPermissionを付与できる設計にはしない。

---

# 7. Envoy ext_authz の責務

Envoy `ext_authz` は、リクエスト入口で判断できる横断的な認可を担当する。

## ext_authzで行う判定

- 認証済み主体の確認
- テナント所属確認
- テナントの有効状態確認
- Entitlement確認
- Role／Permission確認
- プロダクト利用可否
- サービス間アクセス
- 単純なScope確認

例:

```text
POST /v1/invoices/{id}/approve

required_entitlement:
  billing.approval

required_permission:
  invoice.approve
```

認可条件:

```text
allow =
authenticated
AND tenant_active
AND tenant_membership_valid
AND entitlement_available
AND permission_available
```

---

## ext_authzに置かない判定

対象リソースや業務状態を読み込まないと判断できない認可は、各プロダクトに残す。

例:

- 対象invoiceが同じテナントか
- 対象invoiceが同じ部署か
- 自分が作成した請求書か
- 承認金額が上限以内か
- invoiceの状態がsubmittedか
- 自己承認ではないか
- 一覧結果の行フィルタ
- レスポンスのフィールドマスク

---

# 8. 二段階認可

認可は、共通認可とドメイン認可の二段階にする。

```text
Request
  ↓
Envoy ext_authz
  ├─ 認証
  ├─ テナント所属
  ├─ Entitlement
  ├─ Permission
  └─ サービス間認可
  ↓
Product API
  ├─ リソース所有関係
  ├─ Scope／Policy
  ├─ 業務状態
  ├─ 業務上限
  └─ フィールド・行制御
```

最終判定:

```text
allow =
platform_authorization
AND domain_authorization
AND business_rules
```

---

# 9. APIとPermissionの紐づけ

各APIルートに必要なEntitlementとPermissionを宣言する。

URL文字列を認可サービス側で解析するのではなく、ルートメタデータとして設定する。

例:

```yaml
authorization:
  product: billing
  resource: invoice
  action: approve
  entitlement: billing.approval
  permission: invoice.approve
```

認可サービスへ渡す情報:

```json
{
  "subject": {
    "type": "user",
    "id": "user-123",
    "tenant_id": "tenant-456"
  },
  "product": "billing",
  "resource": "invoice",
  "action": "approve",
  "entitlement": "billing.approval",
  "permission": "invoice.approve",
  "context": {
    "route": "ApproveInvoice",
    "method": "POST"
  }
}
```

---

# 10. サービス間認可

ユーザーだけでなく、サービスもSubjectとして扱う。

```json
{
  "subject": {
    "type": "service",
    "id": "billing-service"
  },
  "permission": "customer.read_internal"
}
```

サービス間では、mTLS、SPIFFE ID、ワークロードIDなどを使って主体を特定する。

例:

```text
billing-service
  → customer.read_internal

analytics-service
  → invoice.read_summary
```

ユーザー向けPermissionと内部サービス向けPermissionは分ける。

```text
invoice.read
invoice.read_internal
invoice.read_summary
```

---

# 11. データモデル

## Permission

```text
Permission
- id
- key
- display_name
- description
- product_id
- resource
- action
- risk_level
- delegatable
- custom_role_assignable
- scope_type
- status
```

---

## PermissionDependency

```text
PermissionDependency
- permission_id
- required_permission_id
```

例:

```text
invoice.update
requires
invoice.read
```

---

## Entitlement

```text
Entitlement
- id
- key
- display_name
- description
- product_id
- status
```

---

## EntitlementPermission

```text
EntitlementPermission
- entitlement_id
- permission_id
```

---

## TenantEntitlement

```text
TenantEntitlement
- tenant_id
- entitlement_id
- status
- valid_from
- valid_until
- source
```

---

## Role

```text
Role
- id
- tenant_id
- name
- description
- type
- status
- version
- created_by
- created_at
- updated_at
```

`type`:

```text
system
custom
```

---

## RolePermission

```text
RolePermission
- role_id
- permission_id
- scope
- configured
```

---

## UserRole

```text
UserRole
- tenant_id
- user_id
- role_id
- valid_from
- valid_until
```

---

## DelegatablePermission

```text
DelegatablePermission
- tenant_id
- subject_id
- permission_id
- max_scope
```

---

## AuditLog

```text
AuditLog
- id
- tenant_id
- actor_id
- action
- target_type
- target_id
- before
- after
- result
- reason
- created_at
```

---

# 12. 権限管理画面

## 12.1 ロール一覧

表示項目:

```text
- ロール名
- ロール種別
- 説明
- 所属ユーザー数
- Permission数
- 状態
- 最終更新者
- 最終更新日時
```

操作:

```text
- カスタムロール作成
- ロール複製
- ロール編集
- ロール無効化
- ロール削除
- 所属ユーザー確認
```

システムロールは原則として編集・削除不可とする。

---

## 12.2 ロール編集画面

Permissionをプロダクトと業務機能でグルーピングする。

例:

```text
Billing

請求書
☑ 閲覧
☑ 作成
☑ 編集
☐ 削除

承認
☑ 申請
☐ 承認
☐ 承認ルールの編集
```

内部Permission:

```text
invoice.read
invoice.create
invoice.update
invoice.delete
invoice.submit
invoice.approve
approval_rule.update
```

内部キーは通常非表示とし、詳細表示で確認可能にする。

---

# 13. チェックボックスとPermissionの対応

基本は、1つのチェックボックスと1つのPermissionを対応させる。

```text
請求書を閲覧
→ invoice.read

請求書を承認
→ invoice.approve
```

次の条件に該当するPermissionは、必ず個別に表示する。

- 独立してON/OFFする可能性がある
- セキュリティリスクが異なる
- 監査上区別する必要がある
- 業務ロールごとに付与パターンが異なる

特に次は個別にする。

```text
delete
approve
export
refund
manage_roles
manage_users
```

---

## UI上で複数Permissionをまとめるケース

内部Permissionが常に一緒に使われ、単独付与に意味がない場合は、1つのUI項目にまとめてもよい。

例:

```text
ダッシュボードを利用
  → dashboard.read
  → dashboard.widget.read
  → dashboard.preference.read
```

この場合は、Permissionバンドルを定義する。

```yaml
bundle: dashboard.use
display_name: ダッシュボードを利用

permissions:
  - dashboard.read
  - dashboard.widget.read
  - dashboard.preference.read
```

ただし、高権限Permissionをバンドルへ暗黙に含めない。

---

# 14. Permission依存関係

Permission間に依存関係を持てるようにする。

例:

```text
invoice.update
requires
invoice.read
```

UIでは依存Permissionを明示する。

```text
☑ 請求書を編集
☑ 請求書を閲覧
  編集権限に必要なため自動選択
```

依存Permissionを暗黙に隠さず、保存されるPermissionをユーザーが確認できるようにする。

---

# 15. ScopeのUI

Scopeが必要なPermissionは、チェックボックスと選択肢を組み合わせる。

```text
請求書を閲覧
☑ 許可する

対象範囲:
○ 自分の担当のみ
○ 所属部署
● 組織全体
```

内部表現:

```json
{
  "permission": "invoice.read",
  "scope": "organization"
}
```

`own`、`department`、`organization` を独立チェックボックスにはしない。

---

# 16. Permissionの表示状態

権限管理画面では、最低限次の状態を区別する。

## 利用可能・未選択

```text
☐ レポートを出力
```

Roleへ追加可能。

## 利用可能・選択済み

```text
☑ レポートを出力
```

Roleに設定済みで、実効的にも有効。

## 未契約

```text
☐ レポートを出力
  利用するにはReport Export契約が必要です
```

選択不可。

## 設定済みだが現在無効

```text
☑ レポートを出力
  現在は契約がないため無効です
```

`configured=true`、`effective=false`。

## 操作者が委任不可

```text
☐ 監査ログを削除
  この権限を付与する権限がありません
```

表示自体が機密性を持つ場合は非表示にする。

---

# 17. 権限管理画面で変更できるもの

テナント管理者が変更できる対象:

```text
- カスタムロール名
- カスタムロールの説明
- RolePermission
- PermissionのScope
- ユーザーへのRole割り当て
- ロールの有効・無効
```

変更できない対象:

```text
- Entitlement
- 契約状態
- Permissionマスタ
- APIとPermissionの対応
- システム予約Permission
- 委任不可能なPermission
- システムロールの定義
```

権限管理画面の操作で変更される中心データは、次の2つとする。

```text
RolePermission
UserRole
```

---

# 18. ユーザーの実効権限画面

ユーザーごとに、最終的に有効なPermissionを確認できる画面を用意する。

例:

```text
山田 太郎

請求書を閲覧
有効
付与元: 経理担当
Scope: 所属部署

請求書を承認
有効
付与元: 承認者

レポートを出力
無効
付与元: 経理担当
理由: 契約なし
```

複数Roleから同じPermissionが付与されている場合は、すべての付与元を確認できるようにする。

---

# 19. ロール保存API

例:

```http
PUT /v1/roles/{roleId}
```

リクエスト:

```json
{
  "name": "経理担当",
  "description": "請求書の作成と更新を行う",
  "permissions": [
    {
      "permission": "invoice.read",
      "scope": "department"
    },
    {
      "permission": "invoice.create"
    },
    {
      "permission": "invoice.update",
      "scope": "department"
    }
  ],
  "version": 12
}
```

保存時に以下を検証する。

```text
1. 操作者にロール編集権限がある
2. 操作者と対象Roleのテナントが一致する
3. Roleがカスタムロールである
4. Permissionが存在する
5. Permissionが有効である
6. カスタムロールへ付与可能である
7. 操作者がPermissionを委任可能である
8. Scopeが許可範囲内である
9. Permission依存関係を満たす
10. 楽観ロックのversionが一致する
```

---

# 20. 認可結果

ext_authzの内部判定結果は、理由を区別する。

```text
ALLOW

AUTHENTICATION_REQUIRED
TENANT_NOT_FOUND
TENANT_SUSPENDED
TENANT_MEMBERSHIP_MISSING
ENTITLEMENT_MISSING
PERMISSION_MISSING
ROLE_DISABLED
SCOPE_DENIED
SERVICE_ACCESS_DENIED
```

外部レスポンスでは情報漏えいを避けるため、詳細をそのまま返さない。

例:

```http
HTTP 403 Forbidden
```

内部ログには具体的な拒否理由を記録する。

---

# 21. キャッシュと反映

Role、Permission、Entitlementの判定結果はキャッシュ可能とする。

推奨キャッシュキー:

```text
tenant_id
subject_id
permission
scope
authorization_version
```

権限変更時は以下を行う。

```text
RolePermission更新
  ↓
authorization_version更新
  ↓
キャッシュ無効化
  ↓
次回リクエストから新設定を利用
```

JWTにPermission一覧を埋め込む設計は避ける。

JWTには主体情報を保持する。

```text
user_id
tenant_id
session_id
authentication_context
```

Permission、Role、Entitlementはext_authz側で取得・評価する。

これにより、権限剥奪をトークン失効まで待たずに反映できる。

---

# 22. 監査ログ

以下の操作を監査対象とする。

```text
- カスタムロール作成
- カスタムロール更新
- ロール削除
- ロール無効化
- RolePermission追加
- RolePermission削除
- Scope変更
- UserRole追加
- UserRole削除
- 委任権限変更
- 認可拒否
```

例:

```json
{
  "tenant_id": "tenant-456",
  "actor_id": "user-123",
  "action": "ROLE_PERMISSION_ADDED",
  "target_type": "role",
  "target_id": "role-789",
  "before": null,
  "after": {
    "permission": "invoice.approve",
    "scope": "department"
  },
  "result": "SUCCESS",
  "created_at": "2026-08-01T05:30:00Z"
}
```

---

# 23. Permission登録フロー

各プロダクトチームは、Permissionをコードまたはマニフェストとして管理する。

例:

```yaml
product: billing

permissions:
  - key: invoice.read
    display_name: 請求書を閲覧
    resource: invoice
    action: read
    risk_level: low
    delegatable: true
    custom_role_assignable: true
    scope_type: data_range

  - key: invoice.approve
    display_name: 請求書を承認
    resource: invoice
    action: approve
    risk_level: high
    delegatable: true
    custom_role_assignable: true
    requires:
      - invoice.read
```

共通基盤へ登録する際に次を検証する。

```text
- Permissionキーの重複
- 命名規則
- display_nameの有無
- risk_level
- 委任可否
- 依存関係の循環
- Scope定義
- Entitlementとの関連
```

---

# 24. 推奨する初期スコープ

最初のリリースでは、以下に限定する。

## 実装する

```text
- Entitlement
- Permission
- システムロール
- カスタムロール
- UserRole
- RolePermission
- ext_authzによるAPI Permission判定
- テナント所属判定
- サービス間Permission
- 権限管理画面
- 実効権限確認画面
- 監査ログ
- キャッシュ無効化
```

## 後から追加する

```text
- 複雑なABAC
- フィールド単位ポリシー
- 汎用Policy DSL
- Permissionバンドル
- Capability中間層
- 時間帯条件
- 動的条件式のユーザー編集
```

Capabilityは、複数のEntitlementが同じPermission集合を大量に共有する必要が出てから導入する。

初期設計は次で十分とする。

```text
Entitlement
  ↓
Permission

Role
  ↓
Permission
```

---

# 25. 最終的な認可式

ユーザーAPI:

```text
allow =
authenticated
AND tenant_active
AND tenant_membership_valid
AND required_entitlement_enabled
AND required_permission_in_user_roles
AND role_active
AND scope_allowed
AND domain_policy_allowed
AND business_rule_allowed
```

サービス間API:

```text
allow =
workload_authenticated
AND service_permission_allowed
AND destination_policy_allowed
```

---

# 26. 設計原則

1. EntitlementとRoleを直接紐づけない。

2. EntitlementとRoleは、共通のPermissionを介して評価する。

3. ユーザー操作で変更するのは、主にRolePermissionとUserRoleに限定する。

4. API入口で共通化できる認可はext_authzで行う。

5. リソース内容や業務状態に依存する認可は、プロダクト側に残す。

6. Permissionは細かく定義し、UIは必要に応じて分かりやすくまとめる。

7. 高権限操作は、他のPermissionと暗黙にまとめない。

8. 未契約、未付与、委任不可、設定済みだが無効、を画面上で区別する。

9. カスタムロール管理では、Privilege Escalationを必ず防止する。

10. 権限変更は速やかに反映し、すべて監査可能にする。
