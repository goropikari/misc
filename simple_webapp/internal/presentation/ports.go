package presentation

import (
	"context"

	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
)

// AuthService は認証ハンドラーが利用する認証・登録機能を表します。
type AuthService interface {
	Register(ctx context.Context, username domain.Username, password domain.Password) error
	Login(ctx context.Context, username domain.Username, password domain.Password) (*domain.User, string, error)
}

// MemoService はメモハンドラーが利用するメモ操作機能を表します。
type MemoService interface {
	CreateMemo(ctx context.Context, userID domain.UserID, content domain.MemoContent) (domain.MemoID, error)
	GetMemo(ctx context.Context, userID domain.UserID, memoID domain.MemoID) (*domain.Memo, error)
	UpdateMemo(ctx context.Context, userID domain.UserID, memoID domain.MemoID, content domain.MemoContent) error
	DeleteMemo(ctx context.Context, userID domain.UserID, memoID domain.MemoID) error
	ListMemos(ctx context.Context, userID domain.UserID) ([]*domain.Memo, error)
}

// SessionStore はセッション ID とユーザー ID の対応を保存するストアを表します。
type SessionStore interface {
	Save(ctx context.Context, sessionID string, userID domain.UserID) error
	FindUserID(ctx context.Context, sessionID string) (domain.UserID, error)
	Delete(ctx context.Context, sessionID string) error
}
