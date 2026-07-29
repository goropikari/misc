# practice-pg-command

PostgreSQL wire protocol の練習サーバーです。手前のサーバーが問題を出題し、SQL と psql のカタログ問い合わせは内側の PostgreSQL で処理します。

## 起動

```sh
docker compose up --build
```

別ターミナルから接続します。

```sh
PSQLRC="$PWD/psqlrc" psql -h 127.0.0.1 -p 5432 -U practice -d practice
```

問題に従って `\\dt`、`\\d customers`、`\\dt+`、`\\l` を実行してください。psql のバージョンに依存する列構成は、内側の PostgreSQL が直接返します。
