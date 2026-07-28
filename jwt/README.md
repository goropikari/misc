# JWT sample

JWTの発行と検証を学ぶための、標準ライブラリだけで動くGoサンプルです。

認証処理のシーケンス図と詳細は、[JWT認証シーケンス](docs/jwt-sequence.md)を参照してください。

## 起動

```sh
go run .
```

署名鍵は環境変数で変更できます。

```sh
JWT_SECRET='local-secret' go run .
```

## 試す

ログインしてアクセストークンを取得します。

```sh
curl -s http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"password"}'
```

返された `access_token` を使って、認証が必要なAPIを呼びます。

```sh
curl -s http://localhost:8080/me \
  -H "Authorization: Bearer <access_token>"
```

## 学習ポイント

- JWTは `header.payload.signature` の3部分で構成される
- `header` と `payload` は暗号化ではなくBase64URLエンコード
- この例ではHMAC-SHA256（HS256）で署名する
- `/me`では署名、発行者、必須クレーム、有効期限を検証する
- 署名鍵を知らない相手は、ペイロードを書き換えて有効な署名を作れない

このコードは学習用です。本番では十分に強い鍵の管理、HTTPS、短い有効期限、Refresh Token、失効対策、入力制限などを追加してください。
