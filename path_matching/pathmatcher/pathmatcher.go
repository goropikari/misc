// Package pathmatcher provides different implementations of path matching logic.
package pathmatcher

import (
	"net/http"
)

// Handler は HTTP リクエストを処理する関数型です。
type Handler func(http.ResponseWriter, *http.Request)

// Matcher はパスマッチングのためのインターフェースです。
type Matcher interface {
	// Register はパスとメソッドにハンドラを紐付けます。
	Register(path, method string, handler Handler)
	// Match はパスとメソッドが登録されているかを確認し、登録されていればハンドラを実行します。
	Match(path, method string) bool
}
