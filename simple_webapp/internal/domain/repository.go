package domain

import (
	"context"
	"errors"
	"time"
)

//go:generate mockgen -source=$GOFILE -destination=mock/mock_$GOFILE -package=mock

var (
	ErrUserNotFound      = errors.New("ユーザーが見つかりません")
	ErrUserAlreadyExists = errors.New("ユーザーが既に存在します")
	ErrMemoNotFound      = errors.New("メモが見つかりません")
	ErrUnauthorized      = errors.New("権限がありません")
)

// User はシステム内のユーザーを表します。
type User struct {
	ID           UserID
	Username     Username
	PasswordHash string
}

// Memo はユーザーによって作成されたメモを表します。
type Memo struct {
	ID        MemoID
	UserID    UserID
	Content   MemoContent
	CreatedAt time.Time
}

// UserRepository はユーザー情報のデータアクセス操作を定義します。
type UserRepository interface {
	// Create は新しいユーザーをシステムに保存します。
	Create(ctx context.Context, user *User) error
	// FindByUsername はユーザー名でユーザーを取得します。
	FindByUsername(ctx context.Context, username Username) (*User, error)
	// FindByID は一意のIDでユーザーを取得します。
	FindByID(ctx context.Context, id UserID) (*User, error)
}

// MemoRepository はメモ情報のデータアクセス操作を定義します。
type MemoRepository interface {
	// Create は新しいメモを保存します。
	Create(ctx context.Context, memo *Memo) error
	// FindByID はIDでメモを取得します。
	FindByID(ctx context.Context, id MemoID) (*Memo, error)
	// Update は既存のメモの内容を変更します。
	Update(ctx context.Context, memo *Memo) error
	// Delete はシステムからメモを削除します。
	Delete(ctx context.Context, id MemoID) error
	// FindByUserID は特定のユーザーに属するすべてのメモを取得します。
	FindByUserID(ctx context.Context, userID UserID) ([]*Memo, error)
}
