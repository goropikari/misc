# AGENTS.md

## Testing

テスト方針の要点。

- `testify` を使う
- AAA パターンで書く
- `t.Run` を常に使う
- テーブル駆動テストは原則使わない
- テスト関数名は `Test{対象関数名}` を基本にする

## Development

- 全体テスト: `go test ./...`
- format: `make fmt`
- lint: `make lint`
- コードを編集したら `make fmt` と `make lint` でエラーが出ないことも確認する

## Pull Requests

- PR を作るときは `.github/pull_request_template.md` に必ず従う
- PR 本文は GitHub Markdown として正しい記法で書く
- issue 由来の PR では、`Fixes #<issue number>` または `Closes #<issue number>` を PR 本文に含め、merge 時に GitHub が issue を自動 close するようにする
