package presentation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter(t *testing.T) {
	t.Run("定義済みパスへアクセスする場合、対応するハンドラーに到達する", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		auth := &fakeAuthService{}
		memos := &fakeMemoService{}
		sessions := NewSessionManager(store, SessionConfig{CookieName: "study_session"})
		var router http.Handler
		require.NotPanics(t, func() {
			router = NewRouter(NewHandlers(auth, memos, sessions))
		})
		require.NotNil(t, router)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/login", nil)

		// Act
		require.NotPanics(t, func() {
			router.ServeHTTP(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "login")
		assert.Contains(t, recorder.Body.String(), `href="/register"`)
	})
}

func TestHandlers(t *testing.T) {
	t.Run("未認証ユーザーがログイン画面を表示する場合、ログインHTMLとなる", func(t *testing.T) {
		// Arrange
		handlers := newTestHandlers(newFakeSessionStore(), &fakeAuthService{}, &fakeMemoService{})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/login", nil)

		// Act
		require.NotPanics(t, func() {
			handlers.ShowLogin(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "login")
		assert.Contains(t, recorder.Body.String(), `href="/register"`)
	})

	t.Run("未認証ユーザーが登録画面を表示する場合、登録HTMLとなる", func(t *testing.T) {
		// Arrange
		handlers := newTestHandlers(newFakeSessionStore(), &fakeAuthService{}, &fakeMemoService{})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/register", nil)

		// Act
		require.NotPanics(t, func() {
			handlers.ShowRegister(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "register")
		assert.Contains(t, recorder.Body.String(), `method="post" action="/register"`)
		assert.Contains(t, recorder.Body.String(), `name="username"`)
		assert.Contains(t, recorder.Body.String(), `name="password"`)
		assert.Contains(t, recorder.Body.String(), `href="/login"`)
	})

	t.Run("登録フォームが有効な場合、AuthServiceへ委譲してログイン画面へリダイレクトとなる", func(t *testing.T) {
		// Arrange
		auth := &fakeAuthService{}
		handlers := newTestHandlers(newFakeSessionStore(), auth, &fakeMemoService{})
		recorder := httptest.NewRecorder()
		request := newFormRequest(http.MethodPost, "/register", url.Values{
			"username": {"alice"},
			"password": {"password123"},
		})

		// Act
		require.NotPanics(t, func() {
			handlers.Register(recorder, request)
		})

		// Assert
		assert.True(t, auth.registerCalled)
		assert.Equal(t, domain.Username("alice"), auth.registerUsername)
		assert.Equal(t, domain.Password("password123"), auth.registerPassword)
		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/login", recorder.Header().Get("Location"))
	})

	t.Run("登録に失敗する場合、エラーHTMLとなりパスワードは再表示されない", func(t *testing.T) {
		// Arrange
		auth := &fakeAuthService{registerErr: domain.ErrUserAlreadyExists}
		handlers := newTestHandlers(newFakeSessionStore(), auth, &fakeMemoService{})
		recorder := httptest.NewRecorder()
		request := newFormRequest(http.MethodPost, "/register", url.Values{
			"username": {"alice"},
			"password": {"password123"},
		})

		// Act
		require.NotPanics(t, func() {
			handlers.Register(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), domain.ErrUserAlreadyExists.Error())
		assert.NotContains(t, recorder.Body.String(), "password123")
	})

	t.Run("ログインフォームが有効な場合、セッション保存とCookie発行後にメモ一覧へリダイレクトとなる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		userID := mustUserID(t, 10)
		auth := &fakeAuthService{
			loginUser:      &domain.User{ID: userID, Username: mustUsername(t, "alice")},
			loginSessionID: "session-1",
		}
		handlers := newTestHandlers(store, auth, &fakeMemoService{})
		recorder := httptest.NewRecorder()
		request := newFormRequest(http.MethodPost, "/login", url.Values{
			"username": {"alice"},
			"password": {"password123"},
		})

		// Act
		require.NotPanics(t, func() {
			handlers.Login(recorder, request)
		})

		// Assert
		assert.True(t, auth.loginCalled)
		assert.Equal(t, domain.Username("alice"), auth.loginUsername)
		assert.Equal(t, domain.Password("password123"), auth.loginPassword)
		assert.Equal(t, userID, store.saved["session-1"])
		requireCookie(t, recorder.Result(), "study_session")
		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/memos", recorder.Header().Get("Location"))
	})

	t.Run("ログインに失敗する場合、エラーHTMLとなりパスワードは再表示されない", func(t *testing.T) {
		// Arrange
		auth := &fakeAuthService{loginErr: domain.ErrUserNotFound}
		handlers := newTestHandlers(newFakeSessionStore(), auth, &fakeMemoService{})
		recorder := httptest.NewRecorder()
		request := newFormRequest(http.MethodPost, "/login", url.Values{
			"username": {"alice"},
			"password": {"wrongpass"},
		})

		// Act
		require.NotPanics(t, func() {
			handlers.Login(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), domain.ErrUserNotFound.Error())
		assert.NotContains(t, recorder.Body.String(), "wrongpass")
	})

	t.Run("未認証ユーザーがメモ一覧を表示する場合、ログイン画面へリダイレクトとなる", func(t *testing.T) {
		// Arrange
		handlers := newTestHandlers(newFakeSessionStore(), &fakeAuthService{}, &fakeMemoService{})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/memos", nil)

		// Act
		require.NotPanics(t, func() {
			handlers.ListMemos(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/login", recorder.Header().Get("Location"))
	})

	t.Run("認証済みユーザーがメモ一覧を表示する場合、セッション由来ユーザーIDで一覧取得となる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		userID := mustUserID(t, 10)
		store.saved["session-1"] = userID
		memos := &fakeMemoService{listMemos: []*domain.Memo{
			{ID: mustMemoID(t, 1), UserID: userID, Content: mustMemoContent(t, "alice memo")},
		}}
		handlers := newTestHandlers(store, &fakeAuthService{}, memos)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/memos", nil)
		request.AddCookie(&http.Cookie{Name: "study_session", Value: "session-1"})

		// Act
		require.NotPanics(t, func() {
			handlers.ListMemos(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, userID, memos.listUserID)
		assert.Contains(t, recorder.Body.String(), `href="/memos/new"`)
		assert.Contains(t, recorder.Body.String(), "New memo")
		assert.Contains(t, recorder.Body.String(), `method="post" action="/logout"`)
		assert.Contains(t, recorder.Body.String(), `href="/memos/1/edit"`)
		assert.Contains(t, recorder.Body.String(), `method="post" action="/memos/1/delete"`)
		assert.Contains(t, recorder.Body.String(), "alice memo")
		assert.NotContains(t, recorder.Body.String(), "bob memo")
	})

	t.Run("認証済みユーザーがメモ0件の一覧を表示する場合、新規作成とログアウトのみ操作可能となる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		userID := mustUserID(t, 10)
		store.saved["session-1"] = userID
		memos := &fakeMemoService{listMemos: []*domain.Memo{}}
		handlers := newTestHandlers(store, &fakeAuthService{}, memos)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/memos", nil)
		request.AddCookie(&http.Cookie{Name: "study_session", Value: "session-1"})

		// Act
		require.NotPanics(t, func() {
			handlers.ListMemos(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, userID, memos.listUserID)
		assert.Contains(t, recorder.Body.String(), `href="/memos/new"`)
		assert.Contains(t, recorder.Body.String(), `method="post" action="/logout"`)
		assert.NotContains(t, recorder.Body.String(), `/edit"`)
		assert.NotContains(t, recorder.Body.String(), `/delete"`)
	})

	t.Run("認証済みユーザーがメモ作成画面を表示する場合、メモ登録フォームとなる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		store.saved["session-1"] = mustUserID(t, 10)
		handlers := newTestHandlers(store, &fakeAuthService{}, &fakeMemoService{})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/memos/new", nil)
		request.AddCookie(&http.Cookie{Name: "study_session", Value: "session-1"})

		// Act
		require.NotPanics(t, func() {
			handlers.NewMemo(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `method="post" action="/memos"`)
		assert.Contains(t, recorder.Body.String(), `name="content"`)
		assert.Contains(t, recorder.Body.String(), "Create memo")
		assert.Contains(t, recorder.Body.String(), `href="/memos"`)
	})

	t.Run("認証済みユーザーがメモ編集画面を表示する場合、更新フォームと既存本文と一覧リンクが表示される", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		userID := mustUserID(t, 10)
		store.saved["session-1"] = userID
		memos := &fakeMemoService{getMemo: &domain.Memo{
			ID:      mustMemoID(t, 20),
			UserID:  userID,
			Content: mustMemoContent(t, "study memo"),
		}}
		handlers := newTestHandlers(store, &fakeAuthService{}, memos)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/memos/20/edit", nil)
		request.SetPathValue("id", "20")
		request.AddCookie(&http.Cookie{Name: "study_session", Value: "session-1"})

		// Act
		require.NotPanics(t, func() {
			handlers.EditMemo(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, userID, memos.getUserID)
		assert.Equal(t, domain.MemoID(20), memos.getMemoID)
		assert.Contains(t, recorder.Body.String(), `method="post" action="/memos/20"`)
		assert.Contains(t, recorder.Body.String(), `name="content"`)
		assert.Contains(t, recorder.Body.String(), `>study memo</textarea>`)
		assert.Contains(t, recorder.Body.String(), `href="/memos"`)
	})

	t.Run("認証済みユーザーがHTML特殊文字を含むメモ編集画面を表示する場合、本文はエスケープされる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		userID := mustUserID(t, 10)
		store.saved["session-1"] = userID
		rawContent := `<script>alert('x')</script> & memo`
		memos := &fakeMemoService{getMemo: &domain.Memo{
			ID:      mustMemoID(t, 21),
			UserID:  userID,
			Content: mustMemoContent(t, rawContent),
		}}
		handlers := newTestHandlers(store, &fakeAuthService{}, memos)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/memos/21/edit", nil)
		request.SetPathValue("id", "21")
		request.AddCookie(&http.Cookie{Name: "study_session", Value: "session-1"})

		// Act
		require.NotPanics(t, func() {
			handlers.EditMemo(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `&lt;script&gt;alert(&#39;x&#39;)&lt;/script&gt; &amp; memo`)
		assert.NotContains(t, recorder.Body.String(), rawContent)
	})

	t.Run("メモ作成フォームが有効な場合、フォーム上のユーザーIDではなくセッション由来ユーザーIDで作成となる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		userID := mustUserID(t, 10)
		store.saved["session-1"] = userID
		memos := &fakeMemoService{createID: mustMemoID(t, 100)}
		handlers := newTestHandlers(store, &fakeAuthService{}, memos)
		recorder := httptest.NewRecorder()
		request := newFormRequest(http.MethodPost, "/memos", url.Values{
			"content": {"study memo"},
			"user_id": {"999"},
		})
		request.AddCookie(&http.Cookie{Name: "study_session", Value: "session-1"})

		// Act
		require.NotPanics(t, func() {
			handlers.CreateMemo(recorder, request)
		})

		// Assert
		assert.True(t, memos.createCalled)
		assert.Equal(t, userID, memos.createUserID)
		assert.Equal(t, domain.MemoContent("study memo"), memos.createContent)
		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/memos", recorder.Header().Get("Location"))
	})

	t.Run("メモ作成フォームが不正な場合、入力エラーHTMLとなる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		store.saved["session-1"] = mustUserID(t, 10)
		handlers := newTestHandlers(store, &fakeAuthService{}, &fakeMemoService{})
		recorder := httptest.NewRecorder()
		request := newFormRequest(http.MethodPost, "/memos", url.Values{"content": {"   "}})
		request.AddCookie(&http.Cookie{Name: "study_session", Value: "session-1"})

		// Act
		require.NotPanics(t, func() {
			handlers.CreateMemo(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), domain.ErrInvalidContent.Error())
	})

	t.Run("他ユーザーのメモ編集画面を表示する場合、認可エラー応答となる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		store.saved["session-1"] = mustUserID(t, 10)
		memos := &fakeMemoService{getErr: domain.ErrUnauthorized}
		handlers := newTestHandlers(store, &fakeAuthService{}, memos)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/memos/20/edit", nil)
		request.SetPathValue("id", "20")
		request.AddCookie(&http.Cookie{Name: "study_session", Value: "session-1"})

		// Act
		require.NotPanics(t, func() {
			handlers.EditMemo(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusForbidden, recorder.Code)
		assert.Equal(t, domain.MemoID(20), memos.getMemoID)
	})

	t.Run("存在しないメモ編集画面を表示する場合、未検出エラー応答となる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		store.saved["session-1"] = mustUserID(t, 10)
		memos := &fakeMemoService{getErr: domain.ErrMemoNotFound}
		handlers := newTestHandlers(store, &fakeAuthService{}, memos)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/memos/999/edit", nil)
		request.SetPathValue("id", "999")
		request.AddCookie(&http.Cookie{Name: "study_session", Value: "session-1"})

		// Act
		require.NotPanics(t, func() {
			handlers.EditMemo(recorder, request)
		})

		// Assert
		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("ログアウトする場合、セッション削除後にログイン画面へリダイレクトとなる", func(t *testing.T) {
		// Arrange
		store := newFakeSessionStore()
		store.saved["session-1"] = mustUserID(t, 10)
		handlers := newTestHandlers(store, &fakeAuthService{}, &fakeMemoService{})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/logout", nil)
		request.AddCookie(&http.Cookie{Name: "study_session", Value: "session-1"})

		// Act
		require.NotPanics(t, func() {
			handlers.Logout(recorder, request)
		})

		// Assert
		assert.NotContains(t, store.saved, "session-1")
		requireCookie(t, recorder.Result(), "study_session")
		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/login", recorder.Header().Get("Location"))
	})
}

func newTestHandlers(store SessionStore, auth AuthService, memos MemoService) *Handlers {
	return NewHandlers(auth, memos, NewSessionManager(store, SessionConfig{CookieName: "study_session"}))
}

func newFormRequest(method, target string, values url.Values) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

type fakeAuthService struct {
	registerCalled   bool
	registerUsername domain.Username
	registerPassword domain.Password
	registerErr      error

	loginCalled    bool
	loginUsername  domain.Username
	loginPassword  domain.Password
	loginUser      *domain.User
	loginSessionID string
	loginErr       error
}

func (s *fakeAuthService) Register(_ context.Context, username domain.Username, password domain.Password) error {
	s.registerCalled = true
	s.registerUsername = username
	s.registerPassword = password
	return s.registerErr
}

func (s *fakeAuthService) Login(_ context.Context, username domain.Username, password domain.Password) (*domain.User, string, error) {
	s.loginCalled = true
	s.loginUsername = username
	s.loginPassword = password
	if s.loginErr != nil {
		return nil, "", s.loginErr
	}
	if s.loginUser == nil {
		s.loginUser = &domain.User{ID: 1, Username: username}
	}
	if s.loginSessionID == "" {
		s.loginSessionID = "session-1"
	}
	return s.loginUser, s.loginSessionID, nil
}

type fakeMemoService struct {
	createCalled  bool
	createUserID  domain.UserID
	createContent domain.MemoContent
	createID      domain.MemoID
	createErr     error

	getUserID domain.UserID
	getMemoID domain.MemoID
	getMemo   *domain.Memo
	getErr    error

	updateUserID  domain.UserID
	updateMemoID  domain.MemoID
	updateContent domain.MemoContent
	updateErr     error

	deleteUserID domain.UserID
	deleteMemoID domain.MemoID
	deleteErr    error

	listUserID domain.UserID
	listMemos  []*domain.Memo
	listErr    error
}

func (s *fakeMemoService) CreateMemo(_ context.Context, userID domain.UserID, content domain.MemoContent) (domain.MemoID, error) {
	s.createCalled = true
	s.createUserID = userID
	s.createContent = content
	if s.createErr != nil {
		return 0, s.createErr
	}
	if s.createID == 0 {
		s.createID = 1
	}
	return s.createID, nil
}

func (s *fakeMemoService) GetMemo(_ context.Context, userID domain.UserID, memoID domain.MemoID) (*domain.Memo, error) {
	s.getUserID = userID
	s.getMemoID = memoID
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.getMemo == nil {
		s.getMemo = &domain.Memo{ID: memoID, UserID: userID, Content: domain.MemoContent("study memo")}
	}
	return s.getMemo, nil
}

func (s *fakeMemoService) UpdateMemo(_ context.Context, userID domain.UserID, memoID domain.MemoID, content domain.MemoContent) error {
	s.updateUserID = userID
	s.updateMemoID = memoID
	s.updateContent = content
	return s.updateErr
}

func (s *fakeMemoService) DeleteMemo(_ context.Context, userID domain.UserID, memoID domain.MemoID) error {
	s.deleteUserID = userID
	s.deleteMemoID = memoID
	return s.deleteErr
}

func (s *fakeMemoService) ListMemos(_ context.Context, userID domain.UserID) ([]*domain.Memo, error) {
	s.listUserID = userID
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.listMemos, nil
}

func mustUsername(t *testing.T, value string) domain.Username {
	t.Helper()
	username, err := domain.NewUsername(value)
	require.NoError(t, err)
	return username
}

func mustMemoID(t *testing.T, value int64) domain.MemoID {
	t.Helper()
	memoID, err := domain.NewMemoID(value)
	require.NoError(t, err)
	return memoID
}

func mustMemoContent(t *testing.T, value string) domain.MemoContent {
	t.Helper()
	content, err := domain.NewMemoContent(value)
	require.NoError(t, err)
	return content
}
