# OpenFGA sample

フォルダの権限をドキュメントへ継承する、最小の OpenFGA サンプルです。

## 必要なもの

- Docker Compose
- `curl`
- `python3`
- `jq`
- Docker CLI（DSL の変換に使用）
- Go（FGA CLI を `go install` する場合）

## 実行

```sh
docker compose up -d
./scripts/demo.sh
```

成功すると、次の認可結果が表示されます。

```text
user:alice              viewer   document:roadmap     => True
user:bob                editor   document:roadmap     => True
user:charlie            viewer   document:roadmap     => False
```

この例では [`model.fga`](model.fga) に認可モデルを OpenFGA DSL で定義しています。
実行時に FGA CLI で API 用 JSON へ変換します。

`user:alice` にフォルダの viewer、`user:bob` に editor を付与し、
ドキュメントの `parent` からフォルダの権限を継承させています。

`demo.sh` では、次の relationship tuple を登録しています。

```json
[
  {
    "user": "user:alice",
    "relation": "viewer",
    "object": "folder:engineering"
  },
  {
    "user": "user:bob",
    "relation": "editor",
    "object": "folder:engineering"
  },
  {
    "user": "folder:engineering",
    "relation": "parent",
    "object": "document:roadmap"
  }
]
```

そのため、`alice` は親フォルダ経由でドキュメントを viewer、`bob` は editor として利用できます。

## FGA CLI でモデルを扱う

### インストール

Go がインストールされている環境では、次のコマンドで FGA CLI を導入できます。

```sh
go install github.com/openfga/cli/cmd/fga@latest
export PATH="$(go env GOPATH)/bin:$PATH"
fga version
```

FGA CLI が利用できるようになったら、次のコマンドで DSL モデルを検証・変換できます。

```sh
fga model validate --file model.fga
fga model transform --file model.fga --output-format json
```

OpenFGA が `localhost:8080` で起動している場合は、CLI からモデルを直接登録することもできます。

```sh
export FGA_API_URL=http://localhost:8080
store_id="$(fga store create --name openfga-cli-example --format json | jq -r '.store.id')"
fga model write --store-id "$store_id" --file model.fga
```

`demo.sh` の実行後に表示された store ID と authorization model ID を設定すると、
登録済み tuple を解決した認可チェックを CLI から実行できます。

`alice` の `document:roadmap` に対する viewer は、ドキュメントへ直接登録された tuple ではありません。
次の `parent` 関係をたどって、フォルダの viewer を間接的に解決します。

```text
user:alice viewer folder:engineering
folder:engineering parent document:roadmap
       ↓
user:alice viewer document:roadmap
```

```sh
export FGA_STORE_ID="<demo.sh で表示された store ID>"
export FGA_MODEL_ID="<demo.sh で表示された authorization model ID>"

# 間接参照: folder:engineering の viewer を document:roadmap へ継承
fga query check --store-id "$FGA_STORE_ID" --model-id "$FGA_MODEL_ID" \
  user:alice viewer document:roadmap
# {"allowed":true}

# 間接参照: folder:engineering の editor を document:roadmap へ継承
fga query check --store-id "$FGA_STORE_ID" --model-id "$FGA_MODEL_ID" \
  user:bob editor document:roadmap
# {"allowed":true}

# 関係なし
fga query check --store-id "$FGA_STORE_ID" --model-id "$FGA_MODEL_ID" \
  user:charlie viewer document:roadmap
# {"allowed":false}
```

Playground は http://localhost:3000/playground、HTTP API は http://localhost:8080 で利用できます。

Create store が反応しない場合は、`docker compose up -d` で起動し直してください。
Playground は外部 iframe (`play.fga.dev`) を利用するため、Compose でそのオリジンを CORS 許可しています。

メモリデータストアを使っているため、停止するとデータは消えます。

```sh
docker compose down
```
