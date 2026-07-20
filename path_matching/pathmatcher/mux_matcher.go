// Package pathmatcher provides different implementations of path matching logic.
package pathmatcher

import (
	"net/http"
	"net/url"
)

type muxMatcher struct {
	mux *http.ServeMux
}

// NewMuxMatcher は http.ServeMux を使用した Matcher 実装を生成します。
func NewMuxMatcher() Matcher {
	return &muxMatcher{
		mux: http.NewServeMux(),
	}
}

func (m *muxMatcher) Register(path, method string, handler Handler) {
	m.mux.HandleFunc(method+" "+path, func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	})
}

func (m *muxMatcher) Match(path, method string) bool {
	r := &http.Request{
		Method: method,
		URL:    &url.URL{Path: path},
	}

	w := &writer{
		header: make(http.Header),
	}

	m.mux.ServeHTTP(w, r)

	// http.ServeMux はパターンが一致しない場合に 404 を返します。
	// また、メソッドが一致しない場合は 405 (Method Not Allowed) を返します。
	return w.status != http.StatusNotFound && w.status != http.StatusMethodNotAllowed
}

type writer struct {
	header http.Header
	body   []byte
	status int
}

func (w *writer) Header() http.Header {
	return w.header
}

func (w *writer) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *writer) WriteHeader(statusCode int) {
	w.status = statusCode
}
