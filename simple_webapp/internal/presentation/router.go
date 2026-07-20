package presentation

import "net/http"

// NewRouter は Presentation 層で公開する HTTP ルーティングを構成します。
func NewRouter(handlers *Handlers) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", handlers.ShowLogin)
	mux.HandleFunc("POST /login", handlers.Login)
	mux.HandleFunc("GET /register", handlers.ShowRegister)
	mux.HandleFunc("POST /register", handlers.Register)
	mux.HandleFunc("GET /memos", handlers.ListMemos)
	mux.HandleFunc("POST /memos", handlers.CreateMemo)
	mux.HandleFunc("GET /memos/new", handlers.NewMemo)
	mux.HandleFunc("GET /memos/{id}/edit", handlers.EditMemo)
	mux.HandleFunc("POST /memos/{id}", handlers.UpdateMemo)
	mux.HandleFunc("POST /memos/{id}/delete", handlers.DeleteMemo)
	mux.HandleFunc("POST /logout", handlers.Logout)
	return mux
}
