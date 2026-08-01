# 共通認可基盤 POC

`docs/prd.md` の初期スコープから、認可判定の中核をインメモリで実装しています。

```text
実効Permission = ユーザーのRoleに設定されたPermission
                 ∩ テナントのEntitlementで利用可能なPermission
```

## 実行

```sh
go run ./cmd/sample
```

サンプルでは、Roleに `invoice.approve` が設定されていてもEntitlementを失効させると拒否され、実効権限画面向けの結果には `configured=true` と `ENTITLEMENT_MISSING` が残ることを確認できます。

## パッケージ

- `authz.Authorizer.Evaluate`: ユーザー／サービスの共通認可判定
- `authz.Authorizer.EffectivePermissions`: Role設定を保持した実効権限一覧
- `authz.Decision.Reason`: `ALLOW`、`PERMISSION_MISSING`、`ENTITLEMENT_MISSING` などの内部理由

## 実装状況と優先順位

このリポジトリは認可判定のPOCです。チェック済みは実装済み、未チェックは未実装または今後の拡張です。優先順位は、複雑さよりも利用者が機能を理解しやすいことを重視しています。

### 実装済み

- [x] ユーザー／サービスのPermission認可判定
- [x] テナント所属・有効状態・Entitlementの判定
- [x] Role、RolePermission、UserRoleの基本モデル
- [x] カスタムRoleの作成・更新API
- [x] ユーザーへのRole割り当てAPI
- [x] Permission依存関係、委任可能範囲、楽観ロックの検証
- [x] 実効権限APIと基本画面（設定済み・有効・無効理由の表示）
- [x] Role管理・Role編集の基本画面
- [x] Permissionのプロダクト別管理画面（契約状況・有効状態・ロール設定数・依存関係の表示）
- [x] 認可拒否・Role変更の監査記録と基本的な監査ログ画面
- [x] PostgreSQLへの単一JSONBスナップショット保存

### P0: 本番利用の前提

- [ ] 管理画面のログイン、管理者セッション、CSRF対策、UI操作ごとの追加認証

### P1: 利用者が理解しやすい権限管理

- [ ] Permissionの表示名・説明・プロダクト別グルーピング
- [ ] Permissionをキーの直接入力ではなくチェックボックスで選択するRole編集画面
- [ ] PermissionのScope選択、未契約・委任不可・設定済みだが無効の表示
- [ ] ユーザーへのRole割り当て・解除画面
- [ ] Roleの無効化・削除・複製

### P1: 認可確認用リクエストコンソール

- [ ] Postmanのように、画面からデモプロダクトへの認可付きリクエストを送信できるUI
- [ ] リクエスト先として`demo-a`／`demo-b`のURLとAPIを選択できる機能
- [ ] リクエストに使用するテナント、ユーザー、ユーザーに付与されたRoleを選択できる機能
- [ ] テナントの契約License（Entitlement）の有効／無効を選択できる機能
- [ ] Envoy `ext_authz`の判定結果、HTTPステータス、拒否理由、送信した認可コンテキストを表示する機能
- [ ] 同じ条件で再送信できるリクエスト履歴・リクエスト内容の保存機能

このUIでは、選択したRoleとLicenseを使って認可判定を確認できるようにする。例えば、RoleにPermissionが設定されていてもLicenseを無効にした場合に、`ENTITLEMENT_MISSING`として拒否されることを画面上で確認できるようにする。

### P2: 認可設定と運用の拡張

- [ ] Permission／Entitlement／システムRoleのYAMLマニフェスト登録・検証フロー
- [ ] APIルートの認可メタデータ管理と自動登録
- [x] Envoy `ext_authz` の実際のHTTPアダプターとEnvoy設定
- [ ] 監査ログの検索・ページング・保持期間管理・外部監査基盤連携
- [ ] UserRoleの有効期間、削除・無効化API、Roleの削除・無効化API

### P3: 複数インスタンス・本番基盤対応

- [ ] PostgreSQLの正規化テーブル分割
- [ ] PostgreSQLのマルチインスタンス間キャッシュ、Pub/Subによるキャッシュ無効化、変更通知
- [ ] サービス間認可の証明書・ワークロードID検証
- [ ] PostgreSQLを実際に起動した統合テスト

### P4: 複雑な認可モデル（初期スコープ外）

- [ ] 複雑なABAC、フィールド単位ポリシー、汎用Policy DSL、時間帯条件、ユーザー編集可能な動的条件式
- [ ] Permissionバンドル、Capability中間層
- [ ] リソース所有者、部署、金額上限、業務状態、自己承認などのドメイン／業務ルール
- [ ] 一覧の行フィルタ、レスポンスのフィールドマスク

## PostgreSQLで起動

```sh
docker compose up -d postgres
export AUTHZ_DATABASE_URL='postgres://authz:authz@localhost:5432/authz?sslmode=disable'
go run ./cmd/authz
```

PostgreSQL 17.7を使い、状態・監査ログ・認可バージョンをトランザクション単位で保存します。APIは `POST /v1/authz/check`、`POST/PUT /v1/roles`、`POST /v1/user-roles`、`GET /v1/users/{userID}/permissions`、`GET /v1/audit` を提供します。

管理画面は次のURLから利用できます（`tenant_id` は対象テナントに置き換えてください）。

```text
http://localhost:8080/admin/roles?tenant_id=tenant-1
http://localhost:8080/admin/permissions?tenant_id=tenant-1
http://localhost:8080/admin/permissions?tenant_id=tenant-1&product=billing
http://localhost:8080/admin/audit?tenant_id=tenant-1
http://localhost:8080/admin/users/user-1/permissions?tenant_id=tenant-1
```

ロール編集画面の保存処理は、APIと同じくPermission依存関係、委任可能範囲、楽観ロックを検証します。

## 2つのdemoプロダクトをComposeで起動

`demo-a` と `demo-b` は同じ認可サービスを利用し、それぞれ固有のPermission（`demo-a.dashboard.view`、`demo-b.dashboard.view`）を判定します。Envoyは `localhost:8081` で公開し、パスごとにプロダクトへルーティングします。

```sh
docker compose up --build
```

別ターミナルから、デフォルトの `demo-user` は両方のダッシュボードを利用できます。

```sh
curl -i -H 'X-User-ID: demo-user' -H 'X-Tenant-ID: demo-tenant' http://localhost:8081/demo-a/api/dashboard
curl -i -H 'X-User-ID: demo-user' -H 'X-Tenant-ID: demo-tenant' http://localhost:8081/demo-b/api/dashboard
```

別ユーザーや別テナントを指定すると、認可サービスの拒否理由付き `403` を確認できます。

```sh
curl -i -H 'X-User-ID: unknown-user' -H 'X-Tenant-ID: demo-tenant' http://localhost:8081/demo-a/api/dashboard
curl -i -H 'X-User-ID: demo-user' -H 'X-Tenant-ID: unknown-tenant' http://localhost:8081/demo-b/api/dashboard
```

認可サービスの監査ログは次で確認できます。

```sh
curl http://localhost:8081/v1/audit?tenant_id=demo-tenant

# 管理画面
open http://localhost:8081/admin/permissions?tenant_id=demo-tenant
```

demoプロダクト自身は認可サービスを呼び出しません。EnvoyのHTTP `ext_authz` がリクエストごとにauthzへ問い合わせ、許可された場合だけdemoプロダクトへ転送します。
