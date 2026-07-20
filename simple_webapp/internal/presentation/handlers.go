package presentation

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"

	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
)

// Handlers は Web UI の各 HTTP ハンドラーが利用する依存関係を保持します。
type Handlers struct {
	auth     AuthService
	memos    MemoService
	sessions *SessionManager
}

// NewHandlers は認証、メモ操作、セッション管理を受け取って Handlers を作成します。
func NewHandlers(auth AuthService, memos MemoService, sessions *SessionManager) *Handlers {
	return &Handlers{auth: auth, memos: memos, sessions: sessions}
}

// ShowLogin はログイン画面を表示します。
func (h *Handlers) ShowLogin(w http.ResponseWriter, r *http.Request) {
	renderPage(w, http.StatusOK, "login", "", nil)
}

// ShowRegister はユーザー登録画面を表示します。
func (h *Handlers) ShowRegister(w http.ResponseWriter, r *http.Request) {
	renderPage(w, http.StatusOK, "register", "", nil)
}

// Register は登録フォームを受け取り、AuthService へユーザー登録を委譲します。
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	username, password, err := credentialsFromRequest(r)
	if err != nil {
		renderPage(w, http.StatusBadRequest, "register", err.Error(), nil)
		return
	}
	if err := h.auth.Register(r.Context(), username, password); err != nil {
		renderPage(w, statusForError(err), "register", err.Error(), nil)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Login はログインフォームを受け取り、認証成功時にセッション Cookie を発行します。
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	username, password, err := credentialsFromRequest(r)
	if err != nil {
		renderPage(w, http.StatusBadRequest, "login", err.Error(), nil)
		return
	}

	user, sessionID, err := h.auth.Login(r.Context(), username, password)
	if err != nil {
		renderPage(w, http.StatusBadRequest, "login", err.Error(), nil)
		return
	}
	if err := h.sessions.Create(w, r, sessionID, user.ID); err != nil {
		renderPage(w, http.StatusInternalServerError, "login", err.Error(), nil)
		return
	}
	http.Redirect(w, r, "/memos", http.StatusSeeOther)
}

// ListMemos は認証済みユーザーのメモ一覧画面を表示します。
func (h *Handlers) ListMemos(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	memos, err := h.memos.ListMemos(r.Context(), userID)
	if err != nil {
		renderPage(w, statusForError(err), "memos", err.Error(), nil)
		return
	}
	renderPage(w, http.StatusOK, "memos", "", memos)
}

// NewMemo はメモ作成画面を表示します。
func (h *Handlers) NewMemo(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireUser(w, r); !ok {
		return
	}
	renderPage(w, http.StatusOK, "new memo", "", nil)
}

// CreateMemo はメモ作成フォームを受け取り、MemoService へ作成を委譲します。
func (h *Handlers) CreateMemo(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	content, err := domain.NewMemoContent(r.FormValue("content"))
	if err != nil {
		renderPage(w, http.StatusBadRequest, "new memo", err.Error(), nil)
		return
	}
	if _, err := h.memos.CreateMemo(r.Context(), userID, content); err != nil {
		renderPage(w, statusForError(err), "new memo", err.Error(), nil)
		return
	}
	http.Redirect(w, r, "/memos", http.StatusSeeOther)
}

// EditMemo は指定されたメモの編集画面を表示します。
func (h *Handlers) EditMemo(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	memoID, err := memoIDFromRequest(r)
	if err != nil {
		renderPage(w, http.StatusBadRequest, "edit memo", err.Error(), nil)
		return
	}
	memo, err := h.memos.GetMemo(r.Context(), userID, memoID)
	if err != nil {
		renderPage(w, statusForError(err), "edit memo", err.Error(), nil)
		return
	}
	renderPage(w, http.StatusOK, "edit memo", "", []*domain.Memo{memo})
}

// UpdateMemo はメモ編集フォームを受け取り、MemoService へ更新を委譲します。
func (h *Handlers) UpdateMemo(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	memoID, err := memoIDFromRequest(r)
	if err != nil {
		renderPage(w, http.StatusBadRequest, "edit memo", err.Error(), nil)
		return
	}
	content, err := domain.NewMemoContent(r.FormValue("content"))
	if err != nil {
		renderPage(w, http.StatusBadRequest, "edit memo", err.Error(), nil)
		return
	}
	if err := h.memos.UpdateMemo(r.Context(), userID, memoID, content); err != nil {
		renderPage(w, statusForError(err), "edit memo", err.Error(), nil)
		return
	}
	http.Redirect(w, r, "/memos", http.StatusSeeOther)
}

// DeleteMemo は指定されたメモの削除を MemoService へ委譲します。
func (h *Handlers) DeleteMemo(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	memoID, err := memoIDFromRequest(r)
	if err != nil {
		renderPage(w, http.StatusBadRequest, "memos", err.Error(), nil)
		return
	}
	if err := h.memos.DeleteMemo(r.Context(), userID, memoID); err != nil {
		renderPage(w, statusForError(err), "memos", err.Error(), nil)
		return
	}
	http.Redirect(w, r, "/memos", http.StatusSeeOther)
}

// Logout は現在のセッションを破棄してログイン画面へ遷移させます。
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.sessions.Destroy(w, r); err != nil {
		renderPage(w, http.StatusInternalServerError, "logout", err.Error(), nil)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handlers) requireUser(w http.ResponseWriter, r *http.Request) (domain.UserID, bool) {
	userID, err := h.sessions.Authenticate(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return 0, false
	}
	return userID, true
}

func credentialsFromRequest(r *http.Request) (domain.Username, domain.Password, error) {
	username, err := domain.NewUsername(r.FormValue("username"))
	if err != nil {
		return "", "", err
	}
	password, err := domain.NewPassword(r.FormValue("password"))
	if err != nil {
		return "", "", err
	}
	return username, password, nil
}

func memoIDFromRequest(r *http.Request) (domain.MemoID, error) {
	rawID := strings.TrimSpace(r.PathValue("id"))
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return 0, domain.ErrInvalidMemoID
	}
	return domain.NewMemoID(id)
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusForbidden
	case errors.Is(err, domain.ErrMemoNotFound), errors.Is(err, domain.ErrUserNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrInvalidUserID),
		errors.Is(err, domain.ErrInvalidMemoID),
		errors.Is(err, domain.ErrInvalidUsername),
		errors.Is(err, domain.ErrInvalidPassword),
		errors.Is(err, domain.ErrInvalidContent),
		errors.Is(err, domain.ErrUserAlreadyExists):
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}

func renderPage(w http.ResponseWriter, status int, title string, message string, memos []*domain.Memo) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	fmt.Fprintf(w, "<!doctype html><html><body><h1>%s</h1>", html.EscapeString(title))
	if message != "" {
		fmt.Fprintf(w, `<p class="error">%s</p>`, html.EscapeString(message))
	}
	renderForm(w, title)
	if title == "edit memo" {
		if len(memos) > 0 {
			renderEditMemoForm(w, memos[0])
		}
		fmt.Fprint(w, "</body></html>")
		return
	}
	for _, memo := range memos {
		if memo == nil {
			continue
		}
		fmt.Fprintf(w, `<article data-memo-id="%d">%s`, memo.ID, html.EscapeString(memo.Content.String()))
		if title == "memos" {
			renderMemoItemActions(w, memo)
		}
		fmt.Fprint(w, `</article>`)
	}
	fmt.Fprint(w, "</body></html>")
}

func renderForm(w http.ResponseWriter, title string) {
	switch title {
	case "register":
		fmt.Fprint(w, `<form method="post" action="/register">`)
		fmt.Fprint(w, `<label>Username <input type="text" name="username" autocomplete="username" required></label>`)
		fmt.Fprint(w, `<label>Password <input type="password" name="password" autocomplete="new-password" required></label>`)
		fmt.Fprint(w, `<button type="submit">Register</button>`)
		fmt.Fprint(w, `</form>`)
		renderRegisterNavigation(w)
	case "login":
		fmt.Fprint(w, `<form method="post" action="/login">`)
		fmt.Fprint(w, `<label>Username <input type="text" name="username" autocomplete="username" required></label>`)
		fmt.Fprint(w, `<label>Password <input type="password" name="password" autocomplete="current-password" required></label>`)
		fmt.Fprint(w, `<button type="submit">Login</button>`)
		fmt.Fprint(w, `</form>`)
		renderLoginNavigation(w)
	case "memos":
		renderMemoListActions(w)
	case "new memo":
		fmt.Fprint(w, `<form method="post" action="/memos">`)
		fmt.Fprint(w, `<label>Content <textarea name="content" required></textarea></label>`)
		fmt.Fprint(w, `<button type="submit">Create memo</button>`)
		fmt.Fprint(w, `</form>`)
		renderNewMemoNavigation(w)
	}
}

// renderLoginNavigation はログイン画面から登録画面へ移動する導線を描画します。
func renderLoginNavigation(w http.ResponseWriter) {
	fmt.Fprint(w, `<p><a href="/register">Register</a></p>`)
}

// renderRegisterNavigation は登録画面からログイン画面へ戻る導線を描画します。
func renderRegisterNavigation(w http.ResponseWriter) {
	fmt.Fprint(w, `<p><a href="/login">Login</a></p>`)
}

// renderMemoListActions はメモ一覧画面全体で利用する操作導線を描画します。
func renderMemoListActions(w http.ResponseWriter) {
	fmt.Fprint(w, `<p><a role="button" href="/memos/new">New memo</a></p>`)
	fmt.Fprint(w, `<form method="post" action="/logout">`)
	fmt.Fprint(w, `<button type="submit">Logout</button>`)
	fmt.Fprint(w, `</form>`)
}

// renderMemoItemActions はメモ一覧画面の各メモに対する操作導線を描画します。
func renderMemoItemActions(w http.ResponseWriter, memo *domain.Memo) {
	if memo == nil {
		return
	}
	fmt.Fprintf(w, `<p><a href="/memos/%d/edit">Edit</a></p>`, memo.ID)
	fmt.Fprintf(w, `<form method="post" action="/memos/%d/delete">`, memo.ID)
	fmt.Fprint(w, `<button type="submit">Delete</button>`)
	fmt.Fprint(w, `</form>`)
}

// renderNewMemoNavigation はメモ作成画面からメモ一覧へ戻る導線を描画します。
func renderNewMemoNavigation(w http.ResponseWriter) {
	renderMemosBackLink(w)
}

// renderEditMemoForm はメモ編集画面で既存本文を確認して更新するフォームを描画します。
func renderEditMemoForm(w http.ResponseWriter, memo *domain.Memo) {
	if memo == nil {
		return
	}
	fmt.Fprintf(w, `<form method="post" action="/memos/%d">`, memo.ID)
	fmt.Fprintf(w, `<label>Content <textarea name="content" required>%s</textarea></label>`, html.EscapeString(memo.Content.String()))
	fmt.Fprint(w, `<button type="submit">Update memo</button>`)
	fmt.Fprint(w, `</form>`)
	renderMemosBackLink(w)
}

// renderMemosBackLink はメモ管理画面からメモ一覧へ戻るリンクを描画します。
func renderMemosBackLink(w http.ResponseWriter) {
	fmt.Fprint(w, `<p><a href="/memos">Back to memos</a></p>`)
}
