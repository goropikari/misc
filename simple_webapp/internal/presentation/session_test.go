package presentation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionManager(t *testing.T) {
	t.Run("セッションを作成する場合、ストア保存と認証Cookie発行となる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		manager := NewSessionManager(store, SessionConfig{
			CookieName: "study_session",
			Secure:     true,
			MaxAge:     time.Hour,
		})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/login", nil)
		userID := mustUserID(t, 10)
		var err error

		// Act
		require.NotPanics(t, func() {
			err = manager.Create(recorder, request, "session-1", userID)
		})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, userID, store.saved["session-1"])

		cookie := requireCookie(t, recorder.Result(), "study_session")
		assert.Equal(t, "session-1", cookie.Value)
		assert.True(t, cookie.HttpOnly)
		assert.True(t, cookie.Secure)
		assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
		assert.Greater(t, cookie.MaxAge, 0)
	})

	t.Run("有効なCookieがある場合、認証済みユーザーIDを返す", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		userID := mustUserID(t, 10)
		store.saved["session-1"] = userID
		manager := NewSessionManager(store, SessionConfig{CookieName: "study_session"})
		request := httptest.NewRequest(http.MethodGet, "/memos", nil)
		request.AddCookie(&http.Cookie{Name: "study_session", Value: "session-1"})
		var got domain.UserID
		var err error

		// Act
		require.NotPanics(t, func() {
			got, err = manager.Authenticate(request)
		})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, userID, got)
	})

	t.Run("Cookieが存在しない場合、未認証エラーとなる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		manager := NewSessionManager(store, SessionConfig{CookieName: "study_session"})
		request := httptest.NewRequest(http.MethodGet, "/memos", nil)
		var err error

		// Act
		require.NotPanics(t, func() {
			_, err = manager.Authenticate(request)
		})

		// Assert
		require.Error(t, err)
	})

	t.Run("セッションを破棄する場合、ストア削除と期限切れCookie発行となる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		userID := mustUserID(t, 10)
		store.saved["session-1"] = userID
		manager := NewSessionManager(store, SessionConfig{CookieName: "study_session"})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/logout", nil)
		request.AddCookie(&http.Cookie{Name: "study_session", Value: "session-1"})
		var err error

		// Act
		require.NotPanics(t, func() {
			err = manager.Destroy(recorder, request)
		})

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, store.saved, "session-1")

		cookie := requireCookie(t, recorder.Result(), "study_session")
		assert.Empty(t, cookie.Value)
		assert.True(t, cookie.HttpOnly)
		assert.LessOrEqual(t, cookie.MaxAge, 0)
	})
}

func requireCookie(t *testing.T, response *http.Response, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	require.Failf(t, "cookie not found", "cookie %q was not set", name)
	return nil
}

type fakeSessionStore struct {
	saved map[string]domain.UserID
}

func newFakeSessionStore() *fakeSessionStore {
	return &fakeSessionStore{saved: map[string]domain.UserID{}}
}

func (s *fakeSessionStore) Save(_ context.Context, sessionID string, userID domain.UserID) error {
	s.saved[sessionID] = userID
	return nil
}

func (s *fakeSessionStore) FindUserID(_ context.Context, sessionID string) (domain.UserID, error) {
	userID, ok := s.saved[sessionID]
	if !ok {
		return 0, errors.New("session not found")
	}
	return userID, nil
}

func (s *fakeSessionStore) Delete(_ context.Context, sessionID string) error {
	delete(s.saved, sessionID)
	return nil
}

func mustUserID(t *testing.T, value int64) domain.UserID {
	t.Helper()
	userID, err := domain.NewUserID(value)
	require.NoError(t, err)
	return userID
}
