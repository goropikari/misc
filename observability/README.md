# Observability UI 比較環境

JSONをstdoutへ出力するデモサーバーを同じDockerホスト上で動かし、Dozzle、OpenObserve、Grafana/Lokiで同じログを見比べるための環境です。DozzleはDockerログを直接表示し、Collectorはサーバー専用の共有JSONLログをOpenObserveとLokiへ転送します。

## 起動

```sh
docker compose up --build -d
```

| UI           | URL                   | ログイン                            |
| ------------ | --------------------- | ----------------------------------- |
| APIサーバーA | http://localhost:8088 | -                                   |
| APIサーバーB | http://localhost:8089 | -                                   |
| Dozzle       | http://localhost:9999 | -                                   |
| OpenObserve  | http://localhost:5080 | `admin@example.com` / `password123` |
| Grafana      | http://localhost:3000 | `admin` / `admin`                   |

イベントを増やすには `curl http://localhost:8088/` または `curl http://localhost:8089/`、エラーを出すには各サーバーの `/error` を実行します。Aは2秒、Bは3秒ごとにランダムな200/404/500ログを出します。OpenObserveの `demo` ストリームで `service.name = 'api-server-a'` または `service.name = 'api-server-b'` を条件にするとサービス別に比較できます。

Dockerコンテナのstdout/stderrは、OpenObserveの **docker** ストリームへ転送します。Dockerログ全体を確認したい場合は、OpenObserveのLogsで `docker` ストリームを選択してください。

### OpenObserveでログを見る

1. `http://localhost:5080` にログインする
2. 左メニューの **Logs** を開く
3. ストリームに **demo** を選ぶ
4. 時間範囲を **Last 15 minutes**（または **Today**）にして検索する

Collector起動直後はOpenObserveの起動待ちが発生するため、最初の数秒間だけ転送がリトライされます。確認用APIは次の通りです。

```sh
curl -u admin@example.com:password123 http://localhost:5080/api/default/streams
```

`demo` ストリームの `doc_num` が増えていれば、UIの時間範囲またはストリーム選択の問題です。

## SigNoz

SigNozは公式の現行Docker導入方式がFoundry管理になっているため、`signoz/casting.yaml`を使って別スタックとして起動します。Foundryの導入後、次を実行してください。

```sh
foundryctl cast -f signoz/casting.yaml
```

生成された `pours/deployment/compose.yaml` を起動し、SigNoz UI（通常は http://localhost:8080）を開きます。SigNozへ同じログを送る場合は、生成されたOpenTelemetry Collectorのログパイプラインに本リポジトリの `collector/config.yaml` の `filelog/docker` receiverを追加し、SigNozのOTLP exporterへ向けます。SigNozはDocker環境で少なくとも4GB程度のメモリを見込んでください。

## 停止・初期化

```sh
docker compose down
docker compose down -v  # 保存したOpenObserve/Loki/Grafanaデータも削除
```

## 見比べる観点

- Dozzle: セットアップ不要で、コンテナ単位のtailが最も手軽か
- OpenObserve: JSONフィールド検索・ログ探索のしやすさ
- Grafana/Loki: LogQL、ラベル設計、ダッシュボードとの組み合わせ
- SigNoz: OpenTelemetry中心でログ・メトリクス・トレースを統合できるか

SigNozの現行インストール手順は公式ドキュメントに合わせています: https://signoz.io/docs/install/docker/
