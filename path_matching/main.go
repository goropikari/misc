// Package main はパスマッチングライブラリのデモンストレーションを提供します。
package main

import (
	"fmt"
	"net/http"
	"pathmatcher/pathmatcher"
)

func main() {
	// --- MuxMatcher: パターンとメソッドをサポート ---
	fmt.Println("=== MuxMatcher ===")
	mux := pathmatcher.NewMuxMatcher()
	mux.Register("/example", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "Hello from MuxMatcher /example!")
	})
	mux.Register("/user/{id}/hoge", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "Hello from MuxMatcher /user/{id}/hoge!")
	})

	fmt.Println("/example with GET:     ", matchunmatch(mux.Match("/example", http.MethodGet)))          // Matched
	fmt.Println("/user/123/hoge with GET:", matchunmatch(mux.Match("/user/123/hoge", http.MethodGet)))   // Matched
	fmt.Println("/user/123/hoge with POST:", matchunmatch(mux.Match("/user/123/hoge", http.MethodPost))) // Unmatched

	// --- SimpleMatcher: 完全一致のみ ---
	fmt.Println("\n=== SimpleMatcher ===")
	simple := pathmatcher.NewSimpleMatcher()
	simple.Register("/example", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "Hello from SimpleMatcher /example!")
	})
	simple.Register("/user/{id}/hoge", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "Hello from SimpleMatcher /user/{id}/hoge!")
	})

	fmt.Println("/example with GET:     ", matchunmatch(simple.Match("/example", http.MethodGet)))          // Matched
	fmt.Println("/user/123/hoge with GET:", matchunmatch(simple.Match("/user/123/hoge", http.MethodGet)))   // Unmatched (SimpleMatcher はパターンをサポートしません)
	fmt.Println("/user/{id}/hoge with GET:", matchunmatch(simple.Match("/user/{id}/hoge", http.MethodGet))) // Matched (完全一致)
	fmt.Println("/user/123/hoge with POST:", matchunmatch(simple.Match("/user/123/hoge", http.MethodPost))) // Unmatched
}

func matchunmatch(x bool) string {
	if x {
		return "Matched"
	}
	return "Unmatched"
}
