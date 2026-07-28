# Cedar 認可サンプル

ドキュメントに対する `View` / `Edit` 認可を Cedar で評価する最小サンプルです。

Cedar のポリシーの読み方は [docs/cedar-reading-guide.md](docs/cedar-reading-guide.md) を参照してください。

## ルール

- ドキュメントの所有者は `View` と `Edit` が許可される
- `readers` に登録されたユーザーは `View` のみ許可される
- 所有者以外は、機密 (`confidential`) ドキュメントを `Edit` できない
- Cedar のデフォルト動作により、どの `permit` にも一致しない要求は拒否される

## Cedar CLI で実行

[Cedar CLI](https://github.com/cedar-policy/cedar/tree/main/cedar-cli) をインストールした後、`01_simple` ディレクトリで実行します。

```sh
cd 01_simple
cedar validate --policies policy.cedar --schema schema.cedarschema
./check.sh
```

`check.sh` は次の結果を確認します。

| principal | action | resource | 結果  |
| --------- | ------ | -------- | ----- |
| alice     | View   | report   | Allow |
| alice     | Edit   | report   | Allow |
| bob       | View   | report   | Allow |
| bob       | Edit   | report   | Deny  |

個別に評価する場合:

```sh
cedar authorize \
  --policies policy.cedar \
  --schema schema.cedarschema \
  --entities entities.json \
  --principal 'User::"bob"' \
  --action 'Action::"Edit"' \
  --resource 'Document::"report"'
```

`policy.cedar` がアプリケーションの認可ルール、`schema.cedarschema` が Cedar 形式の型定義、`entities.json` がアプリケーションから渡す認可データに相当します。

## Amazon Verified Permissions との関係

[Amazon Verified Permissions (AVP)](https://docs.aws.amazon.com/verifiedpermissions/latest/userguide/what-is-avp.html) は、Cedar を使った認可判定を AWS のマネージドサービスとして提供するものです。Cedar と AVP は別の選択肢というより、次の関係です。

```text
Cedar                  認可ポリシー言語・評価エンジン
Amazon Verified        Cedar を利用する AWS の認可サービス
Permissions (AVP)
```

このサンプルではローカルの Cedar CLI が認可判定を行います。

```text
policy.cedar + entities.json
        |
        v
    Cedar CLI
        |
        v
    Allow / Deny
```

AVP を使う場合は、ポリシーを AWS のポリシーストアに登録し、アプリケーションから AVP の認可 API を呼び出します。

```text
policy.cedar       -> AVP の Policy Store に登録
schema             -> AVP の Policy Store に登録
entities.json      -> 認可 API の entities としてリクエストに含める
principal/action/resource/context
                    -> AVP の認可 API に渡す
                              |
                              v
                         Allow / Deny
```

対応関係は次の通りです。

| このサンプル         | Amazon Verified Permissions                            |
| -------------------- | ------------------------------------------------------ |
| `policy.cedar`       | Policy Store に保存する Cedar ポリシー                 |
| `schema.cedarschema` | Policy Store のスキーマ                                |
| `entities.json`      | `IsAuthorized` などの認可 API に渡すエンティティデータ |
| `cedar authorize`    | AVP の認可 API 呼び出し                                |
| `check.sh`           | ローカルでの認可動作確認                               |

`schema.cedarschema` はローカル CLI で読みやすい Cedar 形式です。AVP に登録する場合は、AVP が受け付ける Cedar JSON スキーマ形式へ変換して使用します。AVP ではポリシーストアのスキーマを使って、登録するポリシーを検証できます。[AVP のスキーマ](https://docs.aws.amazon.com/verifiedpermissions/latest/userguide/schema.html)

ローカル CLI は学習・テストに向いており、AVP はポリシーの集中管理や AWS 上のアプリケーションからの認可 API 利用に向いています。どちらを使っても、アプリケーション側では AVP または Cedar の判定結果を受け取り、`Allow` の場合だけ実際の処理を実行します。
