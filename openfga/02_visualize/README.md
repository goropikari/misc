# FGA Lens

OpenFGA の Store に接続し、tuple と最新 authorization model をブラウザで可視化する Go サーバーです。

## 起動

必須環境変数を設定して実行します。

```sh
export OPENFGA_API_URL=https://api.openfga.example
export OPENFGA_STORE_ID=01H...
export OPENFGA_AUTH_TOKEN=...
go run .
```

デフォルトでは `127.0.0.1:8080` で待ち受けます。待受先は環境変数または引数で変更できます。

```sh
OPENFGA_VISUALIZER_ADDR=127.0.0.1:9090 go run .
go run . --addr 127.0.0.1:9090
```

ブラウザで <http://127.0.0.1:8080> を開き、`object`、`relation`、`user` を任意に指定して検索します。1回の取得上限は1,000 tupleで、続きがあれば画面下部のリンクから取得できます。

## Docker

```sh
docker build -t fgalens .
docker run --rm -p 127.0.0.1:8080:8080 \
  -e OPENFGA_API_URL=https://api.openfga.example \
  -e OPENFGA_STORE_ID=01H... \
  -e OPENFGA_AUTH_TOKEN=... \
  fgalens
```

## Docker Compose で動作確認

OpenFGA、サンプルデータ投入、FGA Lens をまとめて起動できます。OpenFGA はメモリストレージを使うため、停止するとデータは消えます。

```sh
docker compose up --build -d
```

ブラウザで <http://127.0.0.1:8080> を開いてください。サンプルの authorization model は `demo/store.fga.yaml` 内の OpenFGA DSL、tuple も同ファイルにあります。初期化には OpenFGA CLI の `store import` を使います。OpenFGA は Compose 内部ネットワークからのみアクセスします。

Check の動作例:

- `user:alice` / `viewer` / `document:design` は、group userset と parent 経由で許可されます。
- `user:alice` / `viewer` / `document:roadmap` は拒否され、起点側と対象側の到達領域が別色で表示されます。

停止・データ削除:

```sh
docker compose down -v
```

トークンはサーバー側だけで保持し、ブラウザには渡しません。認証機能は持たないため、公開ネットワークで使う場合はリバースプロキシなどで保護してください。
