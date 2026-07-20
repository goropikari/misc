# Study Memo Simple Webapp

Go と SQLite で作った、ユーザー登録、ログイン、メモ管理ができるシンプルな Web アプリです。

## Requirements

- Go 1.26.4
- SQLite は `github.com/ncruces/go-sqlite3` を使うため、外部の SQLite サーバは不要です。

## Run

```sh
make server
```

または直接起動します。

```sh
go run ./cmd/server
```

デフォルトでは `:8080` で起動し、カレントディレクトリに `study_memo.db` を作成します。

```text
http://127.0.0.1:8080/login
```

## Configuration

環境変数で起動設定を変更できます。

| Name            | Default         | Description                   |
| --------------- | --------------- | ----------------------------- |
| `ADDR`          | `:8080`         | HTTP server の listen address |
| `DATABASE_PATH` | `study_memo.db` | SQLite database file path     |

例:

```sh
ADDR=:3000 DATABASE_PATH=/tmp/study_memo.db go run ./cmd/server
```

## Routes

| Method | Path                 | Description      |
| ------ | -------------------- | ---------------- |
| `GET`  | `/login`             | ログイン画面     |
| `POST` | `/login`             | ログイン         |
| `GET`  | `/register`          | ユーザー登録画面 |
| `POST` | `/register`          | ユーザー登録     |
| `GET`  | `/memos`             | メモ一覧         |
| `GET`  | `/memos/new`         | メモ作成画面     |
| `POST` | `/memos`             | メモ作成         |
| `GET`  | `/memos/{id}/edit`   | メモ編集画面     |
| `POST` | `/memos/{id}`        | メモ更新         |
| `POST` | `/memos/{id}/delete` | メモ削除         |
| `POST` | `/logout`            | ログアウト       |

## Development

```sh
go test ./...
```

```sh
make fmt
```

```sh
make lint
```

```sh
make mockgen
```

## Architecture

`internal/domain`、`internal/application`、`internal/infrastructure`、`internal/presentation` に分けたレイヤード構成です。

- `domain`: エンティティ、値オブジェクト、repository interface
- `application`: 認証とメモ操作のユースケース
- `infrastructure`: SQLite repository、DB 初期化、セッションストア
- `presentation`: HTTP handler、router、session manager
- `cmd/server`: server 起動と依存関係の組み立て
