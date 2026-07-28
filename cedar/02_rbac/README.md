# Cedar RBAC サンプル

ユーザーとロールの関係を Cedar のエンティティ階層で表現し、ロールに応じて API の操作を許可するサンプルです。

## ルール

- `admin` は `Read` / `Write` / `Delete` ができる
- `editor` は `Read` / `Write` ができる
- `viewer` は `Read` だけできる
- どのロールにも属さないユーザーは拒否される
- `forbid` があるため、`admin` でも `production` リソースは `Delete` できない

## ファイル

- `policy.cedar`: ロールごとの認可ルール
- `schema.cedarschema`: `User` と `Role`、操作の型定義
- `entities.json`: ユーザーがどのロールに属するかを表す認可データ
- `check.sh`: 代表的な Allow / Deny 判定の確認

## Docker で実行

ホストに Cedar CLI をインストールせず、リポジトリ直下の `Dockerfile` で作成したイメージを使います。リポジトリのルートで実行してください。

```sh
docker build -t cedar-authz-sample .

docker run --rm \
  -v "$PWD/02_rbac:/work" \
  -w /work \
  cedar-authz-sample \
  cedar validate --policies policy.cedar --schema schema.cedarschema

docker run --rm \
  -v "$PWD/02_rbac:/work" \
  -w /work \
  cedar-authz-sample \
  bash ./check.sh
```

個別に評価する場合:

```sh
docker run --rm \
  -v "$PWD/02_rbac:/work" \
  -w /work \
  cedar-authz-sample \
  cedar authorize \
  --policies policy.cedar \
  --schema schema.cedarschema \
  --entities entities.json \
  --principal 'User::"alice"' \
  --action 'Action::"Write"' \
  --resource 'Resource::"staging"'
```

ユーザーとロールの所属関係は `entities.json` の `parents` で表します。
たとえば `alice` の親に `editor` を指定すると、ポリシーの `principal in Role::"editor"` に一致します。

```text
User::"alice" -> Role::"editor"
                       |
                       +-- Read / Write
```

アプリケーションでは、ログインユーザーを `principal`、実行する操作を `action`、対象リソースを `resource` として Cedar に渡し、判定が `Allow` の場合だけ処理を実行します。

イメージの詳細や `01_simple` の実行方法は、リポジトリ直下の [README.md](../README.md) を参照してください。
