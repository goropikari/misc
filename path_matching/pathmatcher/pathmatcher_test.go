// Package pathmatcher provides different implementations of path matching logic.
package pathmatcher

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMuxMatcher(t *testing.T) {
	t.Run("完全一致パスをGETする場合、マッチする", func(t *testing.T) {
		// Arrange
		m := NewMuxMatcher()
		handlerRun := false
		m.Register("/example", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
			handlerRun = true
			_, _ = fmt.Fprint(w, "ok")
		})

		// Act
		gotMatched := m.Match("/example", http.MethodGet)

		// Assert
		assert.True(t, gotMatched, "マッチすることを期待します")
		assert.True(t, handlerRun, "ハンドラが実行されることを期待します")
	})

	t.Run("パターン一致パスをGETする場合、マッチする", func(t *testing.T) {
		// Arrange
		m := NewMuxMatcher()
		handlerRun := false
		m.Register("/user/{id}/hoge", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
			handlerRun = true
			_, _ = fmt.Fprint(w, "ok")
		})

		// Act
		gotMatched := m.Match("/user/123/hoge", http.MethodGet)

		// Assert
		assert.True(t, gotMatched, "マッチすることを期待します")
		assert.True(t, handlerRun, "ハンドラが実行されることを期待します")
	})

	t.Run("登録パスとメソッドが異なる場合、マッチしない", func(t *testing.T) {
		// Arrange
		m := NewMuxMatcher()
		handlerRun := false
		m.Register("/example", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
			handlerRun = true
			_, _ = fmt.Fprint(w, "ok")
		})

		// Act
		gotMatched := m.Match("/example", http.MethodPost)

		// Assert
		assert.False(t, gotMatched, "マッチしないことを期待します")
		assert.False(t, handlerRun, "ハンドラが実行されないことを期待します")
	})

	t.Run("登録されていないパスをGETする場合、マッチしない", func(t *testing.T) {
		// Arrange
		m := NewMuxMatcher()
		handlerRun := false
		m.Register("/example", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
			handlerRun = true
			_, _ = fmt.Fprint(w, "ok")
		})

		// Act
		gotMatched := m.Match("/wrong", http.MethodGet)

		// Assert
		assert.False(t, gotMatched, "マッチしないことを期待します")
		assert.False(t, handlerRun, "ハンドラが実行されないことを期待します")
	})
}

func TestSimpleMatcher(t *testing.T) {
	t.Run("完全一致パスをGETする場合、マッチする", func(t *testing.T) {
		// Arrange
		m := NewSimpleMatcher()
		handlerRun := false
		m.Register("/example", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
			handlerRun = true
			_, _ = fmt.Fprint(w, "ok")
		})

		// Act
		gotMatched := m.Match("/example", http.MethodGet)

		// Assert
		assert.True(t, gotMatched, "マッチすることを期待します")
		assert.True(t, handlerRun, "ハンドラが実行されることを期待します")
	})

	t.Run("パターン一致パスをGETする場合、マッチしない", func(t *testing.T) {
		// Arrange
		m := NewSimpleMatcher()
		handlerRun := false
		m.Register("/user/{id}/hoge", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
			handlerRun = true
			_, _ = fmt.Fprint(w, "ok")
		})

		// Act
		gotMatched := m.Match("/user/123/hoge", http.MethodGet)

		// Assert
		assert.False(t, gotMatched, "マッチしないことを期待します")
		assert.False(t, handlerRun, "ハンドラが実行されないことを期待します")
	})

	t.Run("パターン文字列そのものをGETする場合、マッチする", func(t *testing.T) {
		// Arrange
		m := NewSimpleMatcher()
		handlerRun := false
		m.Register("/user/{id}/hoge", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
			handlerRun = true
			_, _ = fmt.Fprint(w, "ok")
		})

		// Act
		gotMatched := m.Match("/user/{id}/hoge", http.MethodGet)

		// Assert
		assert.True(t, gotMatched, "マッチすることを期待します")
		assert.True(t, handlerRun, "ハンドラが実行されることを期待します")
	})

	t.Run("登録パスとメソッドが異なる場合、マッチしない", func(t *testing.T) {
		// Arrange
		m := NewSimpleMatcher()
		handlerRun := false
		m.Register("/example", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
			handlerRun = true
			_, _ = fmt.Fprint(w, "ok")
		})

		// Act
		gotMatched := m.Match("/example", http.MethodPost)

		// Assert
		assert.False(t, gotMatched, "マッチしないことを期待します")
		assert.False(t, handlerRun, "ハンドラが実行されないことを期待します")
	})
}
