package domain

import (
	"errors"
	"strings"
)

var (
	ErrInvalidUserID   = errors.New("ユーザーIDが無効です")
	ErrInvalidMemoID   = errors.New("メモIDが無効です")
	ErrInvalidUsername = errors.New("ユーザー名が無効です (3文字以上20文字以内で入力してください)")
	ErrInvalidPassword = errors.New("パスワードが無効です (8文字以上で入力してください)")
	ErrInvalidContent  = errors.New("メモの内容が無効です (1文字以上1000文字以内で入力してください)")
)

// UserID はユーザーの一意な識別子を表す値オブジェクトです。
type UserID int64

// NewUserID はユーザーIDの妥当性を検証し、UserID オブジェクトを作成します。
func NewUserID(id int64) (UserID, error) {
	if id <= 0 {
		return 0, ErrInvalidUserID
	}
	return UserID(id), nil
}

// MemoID はメモの一意な識別子を表す値オブジェクトです。
type MemoID int64

// NewMemoID はメモIDの妥当性を検証し、MemoID オブジェクトを作成します。
func NewMemoID(id int64) (MemoID, error) {
	if id <= 0 {
		return 0, ErrInvalidMemoID
	}
	return MemoID(id), nil
}

// Username はバリデーション済みのユーザー名を表す値オブジェクトです。
type Username string

// NewUsername はユーザー名の妥当性を検証し、Username オブジェクトを作成します。
func NewUsername(s string) (Username, error) {
	s = strings.TrimSpace(s)
	if len(s) < 3 || len(s) > 20 {
		return "", ErrInvalidUsername
	}
	return Username(s), nil
}

// String は文字列としてユーザー名を返します。
func (u Username) String() string {
	return string(u)
}

// Password はバリデーション済みのパスワードを表す値オブジェクトです。
type Password string

// NewPassword はパスワードの妥当性を検証し、Password オブジェクトを作成します。
func NewPassword(s string) (Password, error) {
	if len(s) < 8 {
		return "", ErrInvalidPassword
	}
	return Password(s), nil
}

// String は文字列としてパスワードを返します。
func (p Password) String() string {
	return string(p)
}

// MemoContent はバリデーション済みのメモ内容を表す値オブジェクトです。
type MemoContent string

// NewMemoContent はメモ内容の妥当性を検証し、MemoContent オブジェクトを作成します。
func NewMemoContent(s string) (MemoContent, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || len(s) > 1000 {
		return "", ErrInvalidContent
	}
	return MemoContent(s), nil
}

// String は文字列としてメモ内容を返します。
func (m MemoContent) String() string {
	return string(m)
}
