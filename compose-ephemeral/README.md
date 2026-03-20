# compose-ephemeral

`docker-compose.yaml` の `ports` 指定からホスト側のポート指定を削除し、動的ポート（Ephemeral Port）が割り当てられるようにした `compose.override.yaml` を生成するツールです。

## 背景

Docker Compose で複数のプロジェクトを同時に動かす際、ホスト側のポートが衝突することがあります。このツールを使うと、ホスト側のポート指定をリセットしたオーバライドファイルを生成できるため、Docker が空いているポートを自動的に割り当てる（`docker compose ps` で確認可能）運用が容易になります。

## 使い方

### インストール

```bash
go build -o compose-ephemeral main.go
```

### 実行

対象となる `compose.yaml` を引数に指定します。

```bash
./compose-ephemeral compose.yaml
```

実行後、同じディレクトリに `compose.override.yaml` が生成されます。

### 動的に割り当てられたポートの取得と envrc での活用

`compose-ephemeral` を利用してコンテナを起動すると、ホストのポートは Docker によって動的に割り当てられます。これによってポートの衝突は防げますが、どのポートが割り当てられたかを都度 `docker compose ps` で確認するのは手間がかかります。

そこで `direnv` と `docker compose port` コマンドを組み合わせることで、割り当てられたポート番号を環境変数として簡単に利用できます。

1. **`compose-ephemeral` の実行とコンテナ起動**
   まず、通常通り `compose-ephemeral` を実行し、`compose.override.yaml` を生成してからコンテナを起動します。
   ```bash
   ./compose-ephemeral sample/compose.yaml
   docker compose -f sample/compose.yaml -f compose.override.yaml up -d
   ```

2. **`.envrc` ファイルの作成**
   プロジェクトルートに `.envrc` を作成し、`docker compose port` を使ってポート番号を取得し、環境変数に設定します。

   ```bash
   # .envrc の例
   # docker compose port SERVICE PRIVATE_PORT
   # `docker compose port SERVICE PRIVATE_PORT` は `0.0.0.0:32778` のようなホスト側のアドレスとポートを返すため、
   # そのまま環境変数に設定できます。

   export APP_URL_WEB="http://$(docker compose port web 80)"
   ```
   _`web` はサービス名、`80` はコンテナ側のポート番号です。_

   3. **`direnv allow` の実行**
      `.envrc` を編集したら `direnv allow` を実行します。これにより、`APP_URL_WEB` といった環境変数が自動的に設定されます。

   この方法により、動的ポートを利用する利便性を保ちつつ、API クライアントやテストスクリプトなど、他のツールから簡単にコンテナへアクセスできます。

## 例

### 入力 (compose.yaml)

```yaml
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
  web2:
    image: nginx:latest
    ports:
      - "8081:80"
```

### 出力 (compose.override.yaml)

```yaml
services:
  web:
    ports: !override
      - :80
  web2:
    ports: !override
      - :80
```

このようにホスト側ポートが空（`:80`）になることで、Docker はホストの空きポートを自動選択します。
