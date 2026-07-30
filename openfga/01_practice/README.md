# OpenFGA sample

フォルダの権限をドキュメントへ継承する、最小の OpenFGA サンプルです。

## 必要なもの

- Docker Compose
- `curl`
- `python3`

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

この例では `user:alice` にフォルダの viewer、`user:bob` に editor を付与し、
ドキュメントの `parent` からフォルダの権限を継承させています。

Playground は http://localhost:3000/playground、HTTP API は http://localhost:8080 で利用できます。

Create store が反応しない場合は、`docker compose up -d` で起動し直してください。
Playground は外部 iframe (`play.fga.dev`) を利用するため、Compose でそのオリジンを CORS 許可しています。

メモリデータストアを使っているため、停止するとデータは消えます。

```sh
docker compose down
```
