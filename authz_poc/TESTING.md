# Testing Guidelines

このドキュメントは、Go プロジェクトの開発者向けテスト規約です。テストは実装の詳細を固定するためではなく、コードが提供する振る舞いと契約を保護するために書きます。

## 基本方針

- テストピラミッドを基本とする。まず高速で焦点の狭い単体テストを書き、境界をまたぐ場合に統合テストを追加する。
- 重要なユーザー操作、CLI/API の契約、永続化、状態遷移、エラー処理、境界値を優先する。
- カバレッジの一律の数値目標は設けない。変更のリスクに対して、失敗すべきケースを十分に検証できているかで判断する。
- テストは決定的でなければならない。ローカルとCIで同じ結果になることを重視する。
- テスト名と失敗メッセージから、失敗した振る舞いが分かるようにする。
- 単一のテストから一度だけ呼ばれるだけの薄いテストヘルパーは作らず、テスト本体に直接書く。
- 複数テストで使うセットアップ、リソース管理、フェイク生成などの共通処理はヘルパーに切り出す。

## テストの命名

トップレベルのテスト関数は `TestXXX` 形式とし、`XXX` にはテスト対象の関数またはメソッド名を指定します。

```go
func TestClassify(t *testing.T) { /* ... */ }
func TestCreate(t *testing.T) { /* method Create */ }
```

テストの説明（テスト関数名の補足や `t.Run` のサブテスト名）には、少なくとも「どの前提で」「何を期待するか」を記載します。複数の条件を検証する場合は、`t.Run` のサブテスト名に入力条件と期待結果を記載します。サブテスト名だけで対象を表そうとせず、トップレベルのテスト関数で対象関数・メソッドを明示します。

```go
func TestClassify(t *testing.T) {
	t.Run("when input is empty: returns unknown", func(t *testing.T) {
		// ...
	})
}
```

## テストの選び方

### 単体テスト

純粋なロジック、変換、バリデーション、ポリシー判定など、少数の依存だけで検証できる振る舞いには単体テストを使います。原則として最も多く書くテストです。

### 統合テスト

SQLite、Git、複数の内部サービスなど、実際の境界を組み合わせたときの契約を検証する場合に使います。依存をすべてフェイクに置き換えると見逃す不具合に絞ります。

例:

- SQLite に保存したデータを別のサービスが読み取り、状態遷移を完了できること
- Git の worktree や branch 操作が実際のリポジトリで成立すること
- HTTP ハンドラがルーティング、ステータスコード、JSON の形を満たすこと

### エンドツーエンドテスト

CLI から永続化や実行までの実ユーザーフローなど、複数の境界を通る重要な経路に限定します。高価で失敗原因の特定が難しいため、単体・統合テストの代用にはしません。

## テストの構成

各テストは Arrange–Act–Assert（AAA）で構成します。

各区切りが読み手に分かるよう、テストコード内に `// Arrange`、`// Act`、`// Assert` のコメントを入れます。複数の独立した操作を検証する場合も、準備・実行・検証の境界が曖昧にならないよう、必要に応じてコメントを繰り返します。

1. **Arrange**: 入力、依存、初期状態を準備する。
2. **Act**: 検証対象の振る舞いを一度実行する。
3. **Assert**: 戻り値、エラー、永続化、呼び出しなどの観測可能な結果を検証する。

単一シナリオは通常のテストとして書きます。入力と出力の対応だけで結果が決まる静的・純粋な関数で、入力パターンが複数ある場合に限り、テーブル駆動テストと `t.Run` を使います。状態を持つ対象、外部依存や副作用を伴う対象、実行順序や中間状態を検証する対象は、シナリオごとの個別の `t.Run` として記述します。ケース名には前提と期待結果を含めます。

```go
func TestClassify(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty input is unknown", input: "", want: "unknown"},
		{name: "command input is command", input: "go test ./...", want: "command"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange

			// Act
			got := classify(tt.input)

			// Assert
			require.Equal(t, tt.want, got)
		})
	}
}
```

状態を持つ対象では、テストケースを一つのテーブルに詰め込まず、各シナリオで状態を準備し、実行し、結果と副作用を確認します。

```go
func TestStoreCreate(t *testing.T) {
	t.Run("when the name is unique: creates a record", func(t *testing.T) {
		// Arrange: create an isolated store and input.
		// Act: create the record.
		// Assert: verify the record and persisted state.
	})

	t.Run("when the name already exists: returns a conflict and preserves state", func(t *testing.T) {
		// Arrange: create the existing record.
		// Act: attempt the duplicate creation.
		// Assert: verify the error and unchanged state.
	})
}
```

## アサーション

`github.com/stretchr/testify` を使い、次の基準で `require` と `assert` を使い分けます。

- `require`: セットアップ、エラー確認、nil でないこと、後続の検証に必要な前提条件。
- `assert`: 前提条件に依存しない複数の期待値を一つのテスト内で確認する場合。
- エラーは成功を期待する場合も失敗を期待する場合も検証する。エラーの種類やメッセージが契約なら、それも明示的に確認する。
- 実装内部の一時変数や呼び出し順など、公開された振る舞いに影響しない詳細は検証しない。

```go
got, err := service.Create(ctx, input)
require.NoError(t, err, "Create()")
require.NotNil(t, got)

assert.Equal(t, wantName, got.Name)
assert.Equal(t, wantStatus, got.Status)
```

## 依存の差し替え

外部依存や副作用を単体テストから切り離す場合は、インターフェースに適合する小さな手書きフェイクを基本とします。フェイクはテストに必要な状態と記録だけを持たせ、汎用的なモックフレームワークや過剰な期待値設定は導入しません。

```go
type fakePublisher struct {
	created []string
	err     error
}

func (f *fakePublisher) CreatePullRequest(_ context.Context, title, body, _ string) (string, error) {
	f.created = append(f.created, title)
	if f.err != nil {
		return "", f.err
	}
	return "https://example.test/pull/1", nil
}
```

フェイクでは、結果だけでなく必要な副作用も検証します。たとえば、呼び出し回数、重要な引数、再試行の有無、公開状態の更新などです。ただし、実装の内部手順そのものを固定しないようにします。

SQLite や Git の実際の挙動が対象の場合は、適切な統合テストで実物を使います。テスト対象の境界を隠すためだけにフェイクを使わないでください。

## 一時データとクリーンアップ

- ファイル、データベース、Git リポジトリは `t.TempDir()` の下に作る。
- テスト間で作業ディレクトリ、環境変数、グローバル状態を共有しない。
- リソースは作成直後に `t.Cleanup` で解放する。
- テスト終了後も残るローカルDB、生成 worktree、認証情報をリポジトリに作らない。
- `t.Helper()` をテストヘルパーに付け、失敗箇所が呼び出し元に表示されるようにする。

```go
func openTestStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(t.Context(), store.Options{
		DBPath: filepath.Join(t.TempDir(), "test.db"),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})
	return st
}
```

## 決定性と並行性

- 固定時間の `time.Sleep` で処理の完了を待たない。チャネル、`sync.WaitGroup`、コンテキスト、明示的な完了通知を使う。
- ネットワーク、現在時刻、乱数、OS環境に依存するテストは、依存を注入するか、テスト専用の制御可能な実装を使う。
- `t.Parallel` は、共有状態がなく、テストの独立性を確認できる場合だけ使う。標準設定にはしない。
- 並行処理を変更した場合は、データ競合だけでなく、リーク、デッドロック、キャンセル、終了処理も検証する。
- 競合検出を含む `go test -race ./...` を必ず実行する。

## 失敗系と境界値

正常系だけでなく、次のようなケースを変更のリスクに応じて追加します。

- 空、最小、最大、重複、未知、形式不正の入力
- nil、タイムアウト、キャンセル、部分的な応答
- 外部コマンドや外部サービス連携の失敗、再試行可能な失敗、再試行してはいけない失敗
- SQLite の既存データ、再実行、競合、途中終了
- 認証・認可、秘密情報や個人情報のログ出力
- 状態遷移の各分岐と、処理が途中で失敗した後の再開・取消し

エラー系テストでは、単にエラーが返ることだけでなく、状態が安全に保たれること、不要な副作用が起きないこと、利用者が復旧可能な情報を得られることも確認します。

## 実行と変更時の必須チェック

すべての変更で、提出前に次を実行します。

プロジェクトで定義されたチェックコマンドを実行します。コマンドが用意されていない場合の基本例は次のとおりです。

```sh
go test ./...
go test -race ./...
go vet ./...
```

フォーマット、lint、脆弱性チェックをプロジェクトの標準コマンドに含めます。ローカルで失敗した場合は、失敗を無視して提出せず、原因または再現条件を変更説明に記録します。

変更の説明には、少なくとも次を含めます。

- 追加・変更した振る舞い
- 追加した正常系、失敗系、境界値のテスト
- 実行したコマンドと結果
- 未検証のリスク、環境依存、既知の制約

## テスト作成チェックリスト

- [ ] テストは振る舞いまたは公開契約を検証している
- [ ] 単体テストで検証できるものを不必要に統合テストにしていない
- [ ] AAA の構造になっている
- [ ] テーブル駆動テストは静的・純粋な関数に限定し、状態を持つ対象は個別の `t.Run` で書いている
- [ ] 正常系だけでなく、重要な失敗系・境界値を確認している
- [ ] テスト間で状態やファイルを共有していない
- [ ] 固定 `Sleep` や不安定な外部環境に依存していない
- [ ] 必要な依存は小さな手書きフェイクで差し替えている
- [ ] SQLiteやGitなど、実物の挙動が重要な境界は統合テストで確認している
- [ ] プロジェクトの標準チェックと `go test -race ./...` を実行している
