package pathmatcher

import (
	"net/http"
	"net/url"
)

type simpleMatcher struct {
	handlers map[string]Handler
}

// NewSimpleMatcher は単純なマップによる完全一致マッチングを行う Matcher 実装を生成します。
func NewSimpleMatcher() Matcher {
	return &simpleMatcher{
		handlers: make(map[string]Handler),
	}
}

func (m *simpleMatcher) Register(path, method string, handler Handler) {
	key := method + ":" + path
	m.handlers[key] = handler
}

func (m *simpleMatcher) Match(path, method string) bool {
	key := method + ":" + path
	handler, ok := m.handlers[key]
	if !ok {
		return false
	}

	r := &http.Request{
		Method: method,
		URL:    &url.URL{Path: path},
	}

	w := &simpleWriter{
		header: make(http.Header),
	}

	handler(w, r)
	return true
}

type simpleWriter struct {
	header http.Header
	body   []byte
}

func (w *simpleWriter) Header() http.Header {
	return w.header
}

func (w *simpleWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *simpleWriter) WriteHeader(_ int) {}
