package presentation

import (
	"errors"
	"net/http"
	"time"

	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
)

var ErrUnauthenticated = errors.New("未認証です")

// SessionConfig はセッション Cookie の発行条件を表します。
type SessionConfig struct {
	CookieName string
	Secure     bool
	MaxAge     time.Duration
}

// SessionManager は Cookie とセッションストアを使ってログイン状態を管理します。
type SessionManager struct {
	store  SessionStore
	config SessionConfig
}

// NewSessionManager はセッションストアと Cookie 設定を受け取って SessionManager を作成します。
func NewSessionManager(store SessionStore, config SessionConfig) *SessionManager {
	if config.CookieName == "" {
		config.CookieName = "session"
	}
	return &SessionManager{store: store, config: config}
}

// Create はセッション ID とユーザー ID を保存し、認証用 Cookie をレスポンスへ設定します。
func (m *SessionManager) Create(w http.ResponseWriter, r *http.Request, sessionID string, userID domain.UserID) error {
	if sessionID == "" {
		return ErrUnauthenticated
	}
	if err := m.store.Save(r.Context(), sessionID, userID); err != nil {
		return err
	}

	http.SetCookie(w, m.cookie(sessionID, cookieMaxAge(m.config.MaxAge)))
	return nil
}

// Authenticate はリクエストの Cookie を検証し、認証済みユーザー ID を返します。
func (m *SessionManager) Authenticate(r *http.Request) (domain.UserID, error) {
	cookie, err := r.Cookie(m.config.CookieName)
	if err != nil || cookie.Value == "" {
		return 0, ErrUnauthenticated
	}

	userID, err := m.store.FindUserID(r.Context(), cookie.Value)
	if err != nil {
		return 0, ErrUnauthenticated
	}
	return userID, nil
}

// Destroy は現在のセッションをストアから削除し、期限切れ Cookie をレスポンスへ設定します。
func (m *SessionManager) Destroy(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(m.config.CookieName)
	if err == nil && cookie.Value != "" {
		if err := m.store.Delete(r.Context(), cookie.Value); err != nil {
			return err
		}
	}

	expired := m.cookie("", -1)
	expired.Expires = time.Unix(0, 0)
	http.SetCookie(w, expired)
	return nil
}

func (m *SessionManager) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     m.config.CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.config.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}

func cookieMaxAge(duration time.Duration) int {
	if duration <= 0 {
		return int((24 * time.Hour).Seconds())
	}
	return int(duration.Seconds())
}
