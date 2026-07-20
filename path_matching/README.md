# パスマッチングの学習

Go で HTTP メソッドとパスを使ったルーティングを試すためのサンプルプロジェクトです。
同じ `Matcher` インターフェースに対して、標準ライブラリの `http.ServeMux` を使う実装と、
`map` による完全一致実装を用意しています。

## 実装

| 実装            | マッチ方式                               | 用途                             |
| --------------- | ---------------------------------------- | -------------------------------- |
| `MuxMatcher`    | `http.ServeMux` によるパターンマッチング | パスパラメータを含むルーティング |
| `SimpleMatcher` | `method:path` の完全一致                 | 固定パスのシンプルなルーティング |

どちらも HTTP メソッドでルートを絞り込み、マッチした場合は登録済みのハンドラを実行します。

### MuxMatcher

`http.ServeMux` のルーティング機能を利用します。Go 1.22 以降のパターン記法に対応しているため、
`/user/{id}/hoge` を登録すると `/user/123/hoge` のようなパスにマッチします。

### SimpleMatcher

登録したメソッドとパスの文字列が完全に一致した場合だけマッチします。
`/user/{id}/hoge` はパターンとして解釈されず、その文字列自体にだけマッチします。

## API

```go
type Handler func(http.ResponseWriter, *http.Request)

type Matcher interface {
	Register(path, method string, handler Handler)
	Match(path, method string) bool
}
```

- `Register`: パス、HTTP メソッド、ハンドラを登録します。
- `Match`: パスと HTTP メソッドが登録済みのルートにマッチするかを判定します。マッチした場合はハンドラも実行します。

## 使い方

```go
package main

import (
	"fmt"
	"net/http"

	"pathmatcher/pathmatcher"
)

func main() {
	mux := pathmatcher.NewMuxMatcher()
	mux.Register("/example", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "ok")
	})
	mux.Register("/user/{id}/hoge", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "user")
	})

	fmt.Println(mux.Match("/example", http.MethodGet))          // true
	fmt.Println(mux.Match("/user/123/hoge", http.MethodGet))   // true
	fmt.Println(mux.Match("/user/123/hoge", http.MethodPost))  // false

	simple := pathmatcher.NewSimpleMatcher()
	simple.Register("/example", http.MethodGet, func(http.ResponseWriter, *http.Request) {})
	fmt.Println(simple.Match("/example", http.MethodGet))       // true
}
```

実行可能なデモは [`main.go`](./main.go) にあります。

## 実行

```sh
go run .
go test ./...
```

## 要件

- Go 1.26.4 以降（`go.mod` の指定に準拠）
