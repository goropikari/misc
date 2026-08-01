# Phase Design

- **Phases**: 4

## Phase 1: P0 管理画面の認証とCSRF対策

- **Phase Type**: security
- **Depends On**: none

管理画面にログイン、セッション、CSRF保護、操作時の再認証を追加する。

## Phase 2: P1 権限管理UIの不足機能

- **Phase Type**: feature
- **Depends On**: phase1

Permission表示メタデータ、チェックボックス編集、Scope・利用可否表示、ユーザーRole操作、Roleの無効化・削除・複製を追加する。

## Phase 3: P1 認可確認リクエストコンソール

- **Phase Type**: feature
- **Depends On**: phase1

demo-a/demo-bへの認可付きリクエストを送信するUI、条件選択、結果・コンテキスト表示、履歴を追加する。

## Phase 4: P2-P3 運用・本番基盤

- **Phase Type**: infrastructure
- **Depends On**: phase2, phase3

マニフェスト検証、ルート認可メタデータ、監査検索・保持、Role/UserRole lifecycle、正規化・キャッシュ・証明書検証、PostgreSQL統合テストを追加する。P4の複雑な認可モデルは対象外とする。
