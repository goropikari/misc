# OpenObserve 設定ガイド

このドキュメントでは、このリポジトリの比較環境でOpenObserveを使ってログを収集・検索する方法を説明します。

## 1. 構成

ログの流れは次のとおりです。

```text
APIサーバーA/B
  ├─ stdout(JSON) ───────────────┐
  └─ 共有JSONLファイル            │
                                  ▼
                         OpenTelemetry Collector
                                  │
                                  ▼
                         OpenObserve
```

Collectorは2種類のログをOpenObserveへ送ります。

| ストリーム | 内容                                  |
| ---------- | ------------------------------------- |
| `demo`     | APIサーバーが出力した構造化JSONログ   |
| `docker`   | Dockerコンテナのstdout/stderrログ全体 |

DozzleはDockerログを直接表示しますが、OpenObserveでは保存・検索・集計できます。

## 2. 起動

Docker Composeを起動します。

```sh
docker compose up --build -d
```

起動状態を確認します。

```sh
docker compose ps
```

以下のコンテナが `Up` になれば起動完了です。

- `observe-log-server-a`
- `observe-log-server-b`
- `observe-collector`
- `observe-openobserve`

OpenObserveのURLは次のとおりです。

```text
http://localhost:5080
```

ログイン情報はComposeに設定されています。

```text
ユーザー: admin@example.com
パスワード: password123
```

## 3. ログを発生させる

サーバーは起動後、自動的にログを出力します。

- APIサーバーA: 2秒ごと
- APIサーバーB: 3秒ごと

手動でもログを発生させられます。

```sh
curl http://localhost:8088/
curl http://localhost:8089/
curl http://localhost:8088/error
curl http://localhost:8089/error
```

各サーバーは200、404、500のイベントをランダムに出力します。

## 4. OpenObserveでログを見る

1. `http://localhost:5080` を開く
2. ログインする
3. 左メニューから **Logs** を開く
4. ストリームを選択する
5. 時間範囲を **Last 15 minutes** または **Today** にする

アプリケーションログを見る場合は、ストリームに `demo` を選びます。

Dockerコンテナのログ全体を見る場合は、ストリームに `docker` を選びます。

起動直後はOpenObserveの初期化に時間がかかる場合があります。ログが表示されない場合は、数秒待ってからRefreshしてください。

## 5. `demo` ストリームの検索

JSONフィールドを使ってAPIサーバーAだけに絞り込めます。

```sql
service.name = 'api-server-a'
```

APIサーバーBの場合です。

```sql
service.name = 'api-server-b'
```

500エラーだけを検索する場合です。

```sql
status = 500
```

APIサーバーA/Bの500エラーをまとめて見る場合です。

```sql
status = 500 AND service.name IN ('api-server-a', 'api-server-b')
```

OpenObserveのSQL Modeを使う場合は、次のように集計できます。

```sql
SELECT service.name, status, count(*) AS count
FROM demo
GROUP BY service.name, status
ORDER BY count DESC
```

## 6. ストリームをまたいで検索する

通常のLogs画面は1ストリームずつ検索します。複数ストリームを横断する場合は、SQL Modeで `UNION ALL` を使います。

```sql
SELECT _timestamp, body, 'demo' AS source_stream
FROM demo

UNION ALL

SELECT _timestamp, body, 'docker' AS source_stream
FROM docker

ORDER BY _timestamp DESC
LIMIT 100
```

エラーを横断検索する例です。

```sql
SELECT _timestamp, body, 'demo' AS source_stream
FROM demo
WHERE body LIKE '%error%'

UNION ALL

SELECT _timestamp, body, 'docker' AS source_stream
FROM docker
WHERE body LIKE '%error%'

ORDER BY _timestamp DESC
LIMIT 100
```

## 7. サーバーを追加する

新しいサーバーを追加する場合は、次の3点を分けて設定します。

1. `SERVICE_NAME` に一意な名前を設定する
2. `LOG_FILE` に一意なJSONLファイル名を設定する
3. ホスト側のポートを変更する

Composeの例です。

```yaml
log-server-c:
  build: ./server
  container_name: observe-log-server-c
  user: "0:0"
  environment:
    LOG_INTERVAL: 4s
    SERVICE_NAME: api-server-c
    LOG_FILE: /logs/api-server-c.jsonl
  ports:
    - "8090:8080"
  volumes:
    - app-logs:/logs
```

Collectorの `filelog/demo` は `/logs/*.jsonl` を読むため、新しいJSONLファイルを個別に追加する必要はありません。

## 8. ログの形式

APIサーバーはstdoutと共有JSONLファイルに同じJSONを出力します。

```json
{
  "time": "2026-07-20T11:00:00Z",
  "level": "INFO",
  "msg": "demo request completed",
  "service.name": "api-server-a",
  "method": "GET",
  "path": "/api/items/1",
  "status": 200,
  "latency_ms": 120,
  "request_id": "example-request-id",
  "env": "comparison"
}
```

`service.name`、`status`、`path`、`request_id`などが検索フィールドとして利用できます。

## 9. APIで確認する

ストリーム一覧を確認します。

```sh
curl -u admin@example.com:password123 \
  http://localhost:5080/api/default/streams
```

`demo` と `docker` が表示されれば、CollectorからOpenObserveへの接続は確立しています。

ログ件数やフィールドをSQL APIで調べる場合は、時間範囲をマイクロ秒で指定します。

```sh
curl -u admin@example.com:password123 \
  -H 'Content-Type: application/json' \
  -X POST http://localhost:5080/api/default/_search \
  -d '{
    "query": {
      "sql": "SELECT service.name, status, count(*) FROM demo GROUP BY service.name, status",
      "start_time": 1784500000000000,
      "end_time": 1784600000000000,
      "from": 0,
      "size": 100
    }
  }'
```

## 10. トラブルシュート

### ストリームが表示されない

Collectorのログを確認します。

```sh
docker compose logs --tail=100 collector
```

次のようなログが起動直後に出る場合があります。

```text
connect: connection refused
```

OpenObserveの起動待ちによる一時的なエラーです。Collectorはリトライするため、数秒後に解消します。

### アプリログが表示されない

共有ログファイルが作られているか確認します。

```sh
docker volume inspect observe-compare_app-logs
docker compose logs --tail=20 log-server-a log-server-b
```

Collectorに次のログがあれば、JSONLファイルを検出できています。

```text
Started watching file ... /logs/api-server-a.jsonl
Started watching file ... /logs/api-server-b.jsonl
```

### Dockerログが表示されない

CollectorがDockerログファイルを読むため、Composeでは次のマウントが必要です。

```yaml
- /var/lib/docker/containers:/var/lib/docker/containers:ro
```

また、CollectorはDockerログファイルを読むためrootユーザーで起動しています。

### 時間範囲に注意する

ログ検索には時間範囲が適用されます。ログがあるはずなのに結果がない場合は、まず **Last 15 minutes**、次に **Today** を試してください。

## 11. 停止とデータ削除

コンテナだけ停止します。ログデータは残ります。

```sh
docker compose down
```

ログデータも含めて初期化します。

```sh
docker compose down -v
```

`down -v` はOpenObserve、Loki、Grafana、共有JSONLログのボリュームを削除するため、保存したログは復元できません。

## 12. この構成の使い分け

- まず手軽にtailする: Dozzle
- JSONフィールドで検索・集計する: OpenObserve
- LogQLやダッシュボードを試す: Grafana/Loki
- OpenTelemetryのログ・メトリクス・トレース統合を試す: SigNoz

この比較環境では、OpenObserveを中心にログを集約し、同じイベントをDozzleやGrafana/Lokiでも確認できます。
