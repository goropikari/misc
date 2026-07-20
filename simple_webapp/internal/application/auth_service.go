package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

// AuthService はユーザー認証と登録を処理します。
type AuthService struct {
	userRepo domain.UserRepository
}

// NewAuthService は AuthService の新しいインスタンスを作成します。
func NewAuthService(repo domain.UserRepository) *AuthService {
	return &AuthService{userRepo: repo}
}

// Register はパスワードをハッシュ化して新しいユーザーを作成します。
func (s *AuthService) Register(ctx context.Context, username domain.Username, password domain.Password) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password.String()), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := &domain.User{
		Username:     username,
		PasswordHash: string(hashedPassword),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return err
	}

	return nil
}

// Login はユーザーを認証し、ユーザー情報とセッションIDを返します。
func (s *AuthService) Login(ctx context.Context, username domain.Username, password domain.Password) (*domain.User, string, error) {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, "", err
	}

	valid, err := s.ValidatePassword(password.String(), user.PasswordHash)
	if err != nil {
		return nil, "", err
	}
	if !valid {
		return nil, "", fmt.Errorf("authentication failed: invalid password")
	}

	sessionID, err := s.GenerateSessionID()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate session ID: %w", err)
	}

	return user, sessionID, nil
}

// ValidatePassword は提供されたパスワードが保存されたハッシュと一致するか確認します。
func (s *AuthService) ValidatePassword(password, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GenerateSessionID はユーザーセッションのための一意の識別子を生成します。
func (s *AuthService) GenerateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
