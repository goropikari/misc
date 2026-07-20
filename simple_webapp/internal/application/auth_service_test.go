package application_test

import (
	"context"
	"testing"

	"github.com/goropikari/study_memo/simple_webapp/internal/application"
	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
	"github.com/goropikari/study_memo/simple_webapp/internal/domain/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthService(t *testing.T) {
	ctx := context.Background()

	t.Run("ユーザー名が一意の場合、登録成功となる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockUserRepository(ctrl)
		service := application.NewAuthService(repo)
		username := mustUsername(t, "alice")
		password := mustPassword(t, "correct-password")

		repo.EXPECT().
			Create(ctx, gomock.Any()).
			Return(nil).
			Times(1)

		// Act
		err := service.Register(ctx, username, password)

		// Assert
		require.NoError(t, err)
	})

	t.Run("ユーザー名が既に存在する場合、ユーザー重複エラーとなる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockUserRepository(ctrl)
		service := application.NewAuthService(repo)
		username := mustUsername(t, "alice")
		password := mustPassword(t, "correct-password")

		repo.EXPECT().
			Create(ctx, gomock.Any()).
			Return(domain.ErrUserAlreadyExists).
			Times(1)

		// Act
		err := service.Register(ctx, username, password)

		// Assert
		require.ErrorIs(t, err, domain.ErrUserAlreadyExists)
	})

	t.Run("パスワードが正しい場合、ログイン成功となる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockUserRepository(ctrl)
		service := application.NewAuthService(repo)
		username := mustUsername(t, "alice")
		password := mustPassword(t, "correct-password")

		hash, _ := bcrypt.GenerateFromPassword([]byte(password.String()), bcrypt.DefaultCost)
		user := &domain.User{
			ID:           1,
			Username:     username,
			PasswordHash: string(hash),
		}

		repo.EXPECT().
			FindByUsername(ctx, username).
			Return(user, nil).
			Times(1)

		// Act
		u, sessionID, err := service.Login(ctx, username, password)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, u)
		assert.Equal(t, username, u.Username)
		assert.NotEmpty(t, sessionID)
	})

	t.Run("パスワードが誤っている場合、認証失敗となる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockUserRepository(ctrl)
		service := application.NewAuthService(repo)
		username := mustUsername(t, "alice")
		password := mustPassword(t, "correct-password")

		hash, _ := bcrypt.GenerateFromPassword([]byte(password.String()), bcrypt.DefaultCost)
		user := &domain.User{
			ID:           1,
			Username:     username,
			PasswordHash: string(hash),
		}

		repo.EXPECT().
			FindByUsername(ctx, username).
			Return(user, nil).
			Times(1)

		// Act
		_, _, err := service.Login(ctx, username, mustPassword(t, "wrong-password"))

		// Assert
		require.Error(t, err)
	})

	t.Run("ユーザーが存在しない場合、ユーザー未検出エラーとなる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockUserRepository(ctrl)
		service := application.NewAuthService(repo)
		username := mustUsername(t, "bob")
		password := mustPassword(t, "password123")

		repo.EXPECT().
			FindByUsername(ctx, username).
			Return(nil, domain.ErrUserNotFound).
			Times(1)

		// Act
		_, _, err := service.Login(ctx, username, password)

		// Assert
		require.ErrorIs(t, err, domain.ErrUserNotFound)
	})
}

func mustUsername(t *testing.T, value string) domain.Username {
	t.Helper()
	username, err := domain.NewUsername(value)
	require.NoError(t, err)
	return username
}

func mustPassword(t *testing.T, value string) domain.Password {
	t.Helper()
	password, err := domain.NewPassword(value)
	require.NoError(t, err)
	return password
}
