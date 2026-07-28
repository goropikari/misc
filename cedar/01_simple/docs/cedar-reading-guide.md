# Cedar の読み方

Cedar は「シーダー」と読みます。英語で杉・ヒマラヤスギを意味します。

## 認可リクエストの4要素

Cedar のポリシーは、基本的に次の4要素で読みます。

```text
principal（誰が）
action（何を）
resource（何に対して）
context（どんな条件で）
```

このサンプルでは、例えば次の問い合わせを評価します。

```text
alice が report を View できるか？
```

## ポリシーの読み方

### 所有者の許可

```cedar
permit (
    principal,
    action in [Action::"View", Action::"Edit"],
    resource
)
when { principal == resource.owner };
```

これは次の意味です。

> principal が resource の owner と同じなら、View と Edit を許可する。

### 閲覧者の許可

```cedar
permit (
    principal,
    action == Action::"View",
    resource
)
when { principal in resource.readers };
```

これは次の意味です。

> resource.readers に principal が含まれているなら、View を許可する。

`entities.json` では Bob が readers に登録されています。そのため Bob は閲覧できますが、Edit の許可はありません。

### 明示的な拒否

```cedar
forbid (
    principal,
    action == Action::"Edit",
    resource
)
when {
    resource.classification == "confidential" &&
    principal != resource.owner
};
```

これは、機密文書に対する所有者以外の Edit を拒否するルールです。

## `permit` と `forbid`

Cedar の判定は次のルールです。

```text
forbid に一致                    => Deny
forbid なし、permit に一致       => Allow
どちらにも一致しない             => Deny
```

`forbid` は `permit` より優先されます。また、どの `permit` にも一致しない要求はデフォルトで拒否されます。

Bob の Edit は次のように評価されます。

```text
所有者向け permit       => 不一致
閲覧者向け permit       => action が Edit なので不一致
機密文書向け forbid     => 一致
結果                    => Deny
```

## スキーマの読み方

```cedar
entity User {
    name: String,
};
```

`User` というエンティティがあり、`name` という文字列属性を持つ、という定義です。

```cedar
entity Document {
    owner: User,
    readers: Set<User>,
    classification: String,
};
```

`Document` は `owner`（User 1人）、`readers`（User の集合）、`classification`（文字列）を持ちます。

```cedar
action View, Edit appliesTo {
    principal: User,
    resource: Document,
};
```

View と Edit が User から Document に対して実行されるアクションであることを定義しています。

## CLI コマンドの読み方

```sh
cedar authorize \
  --policies policy.cedar \
  --schema schema.cedarschema \
  --entities entities.json \
  --principal 'User::"bob"' \
  --action 'Action::"Edit"' \
  --resource 'Document::"report"'
```

これは「Bob は report に対して Edit を実行できるか？」という問い合わせです。このサンプルでの結果は `DENY` です。

## 各ファイルの意味

このサンプルは、次のように役割を分けています。

| ファイル             | 役割                                       |
| -------------------- | ------------------------------------------ |
| `policy.cedar`       | 誰が何をできるかという認可ルール           |
| `schema.cedarschema` | エンティティ、属性、アクションの型定義     |
| `entities.json`      | User や Document の実データ                |
| `check.sh`           | Cedar CLI で Allow / Deny を確認するテスト |
| `README.md`          | サンプル全体の概要と実行手順               |

認可判定では、`policy.cedar` だけでは不十分です。Cedar はポリシーを `entities.json` のデータに対して評価し、`schema.cedarschema` でその構造を検証します。

```text
policy.cedar       認可ルール
       +
entities.json      ユーザー・文書データ
       +
schema.cedarschema 型とアクションの契約
       |
       v
     Cedar CLI
       |
       v
    Allow / Deny
```

### `policy.cedar`

Cedar のポリシー本体です。アプリケーションのコードから認可ロジックを分離して管理できます。

### `schema.cedarschema`

ポリシーとアプリケーションデータの契約です。例えば `Document.owner` は `User` であり、`classification` は文字列であることを定義します。

### `entities.json`

認可判定に使うエンティティデータです。この例では Alice、Bob、report 文書を定義しています。実際のアプリケーションでは、データベースや認証情報からこの形のデータを組み立てます。

### `check.sh`

代表的な認可ケースを自動確認するスクリプトです。Alice の View / Edit と Bob の View / Edit を評価し、期待結果と異なる場合は終了コード 1 で失敗します。

## 認証との違い

Cedar は認証ではなく認可を担当します。

```text
認証（Authentication）: その人が本当に Alice か？
認可（Authorization）: Alice はこの文書を編集してよいか？
```

ログインやトークン検証でユーザーを確認した後、アプリケーションが Cedar に認可リクエストを渡します。
