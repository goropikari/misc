package infrastructure_test

import (
	"context"
	"testing"

	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
	"github.com/goropikari/study_memo/simple_webapp/internal/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteUserRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("ユーザーを作成する場合、IDが採番され永続化済みとなる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "user-create")
		repo := infrastructure.NewSQLiteUserRepository(mgr.DB())
		user := &domain.User{
			Username:     mustTestUsername(t, "alice"),
			PasswordHash: "hashed_password_123",
		}

		// Act
		err := repo.Create(ctx, user)

		// Assert
		require.NoError(t, err)
		assert.NotZero(t, user.ID)
		got, err := repo.FindByUsername(ctx, user.Username)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, user.ID, got.ID)
		assert.Equal(t, user.Username, got.Username)
		assert.Equal(t, user.PasswordHash, got.PasswordHash)
	})

	t.Run("重複したユーザー名で作成する場合、ユーザー重複エラーとなる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "user-duplicate")
		repo := infrastructure.NewSQLiteUserRepository(mgr.DB())
		first := &domain.User{Username: mustTestUsername(t, "alice"), PasswordHash: "hash-1"}
		second := &domain.User{Username: mustTestUsername(t, "alice"), PasswordHash: "hash-2"}
		require.NoError(t, repo.Create(ctx, first))

		// Act
		err := repo.Create(ctx, second)

		// Assert
		require.ErrorIs(t, err, domain.ErrUserAlreadyExists)
	})

	t.Run("ユーザー名に一致するユーザーが存在しない場合、ユーザー未検出エラーとなる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "user-missing-username")
		repo := infrastructure.NewSQLiteUserRepository(mgr.DB())

		// Act
		got, err := repo.FindByUsername(ctx, mustTestUsername(t, "missing"))

		// Assert
		require.ErrorIs(t, err, domain.ErrUserNotFound)
		require.Nil(t, got)
	})

	t.Run("ユーザーIDに一致するユーザーが存在する場合、保存済みユーザー取得となる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "user-find-by-id")
		repo := infrastructure.NewSQLiteUserRepository(mgr.DB())
		user := &domain.User{Username: mustTestUsername(t, "alice"), PasswordHash: "hash-1"}
		require.NoError(t, repo.Create(ctx, user))

		// Act
		got, err := repo.FindByID(ctx, user.ID)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, user.ID, got.ID)
		assert.Equal(t, user.Username, got.Username)
		assert.Equal(t, user.PasswordHash, got.PasswordHash)
	})

	t.Run("ユーザーIDに一致するユーザーが存在しない場合、ユーザー未検出エラーとなる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "user-missing-id")
		repo := infrastructure.NewSQLiteUserRepository(mgr.DB())

		// Act
		got, err := repo.FindByID(ctx, mustTestUserID(t, 999))

		// Assert
		require.ErrorIs(t, err, domain.ErrUserNotFound)
		require.Nil(t, got)
	})
}

func TestSQLiteMemoRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("メモを作成する場合、IDが採番されID検索で取得可能となる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "memo-create")
		userRepo := infrastructure.NewSQLiteUserRepository(mgr.DB())
		memoRepo := infrastructure.NewSQLiteMemoRepository(mgr.DB())
		user := &domain.User{Username: mustTestUsername(t, "alice"), PasswordHash: "hash-1"}
		require.NoError(t, userRepo.Create(ctx, user))
		memo := &domain.Memo{
			UserID:    user.ID,
			Content:   mustTestMemoContent(t, "Hello World"),
			CreatedAt: mustTestTime(t, "2026-07-09T12:00:00Z"),
		}

		// Act
		err := memoRepo.Create(ctx, memo)

		// Assert
		require.NoError(t, err)
		assert.NotZero(t, memo.ID)
		got, err := memoRepo.FindByID(ctx, memo.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, memo.ID, got.ID)
		assert.Equal(t, memo.UserID, got.UserID)
		assert.Equal(t, memo.Content, got.Content)
		assert.False(t, got.CreatedAt.IsZero())
	})

	t.Run("メモIDに一致するメモが存在しない場合、メモ未検出エラーとなる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "memo-missing-id")
		repo := infrastructure.NewSQLiteMemoRepository(mgr.DB())

		// Act
		got, err := repo.FindByID(ctx, mustTestMemoID(t, 999))

		// Assert
		require.ErrorIs(t, err, domain.ErrMemoNotFound)
		require.Nil(t, got)
	})

	t.Run("メモ内容を更新する場合、変更後の内容が永続化済みとなる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "memo-update")
		userRepo := infrastructure.NewSQLiteUserRepository(mgr.DB())
		memoRepo := infrastructure.NewSQLiteMemoRepository(mgr.DB())
		user := &domain.User{Username: mustTestUsername(t, "alice"), PasswordHash: "hash-1"}
		require.NoError(t, userRepo.Create(ctx, user))
		memo := &domain.Memo{
			UserID:    user.ID,
			Content:   mustTestMemoContent(t, "before"),
			CreatedAt: mustTestTime(t, "2026-07-09T12:00:00Z"),
		}
		require.NoError(t, memoRepo.Create(ctx, memo))
		memo.Content = mustTestMemoContent(t, "after")

		// Act
		err := memoRepo.Update(ctx, memo)

		// Assert
		require.NoError(t, err)
		got, err := memoRepo.FindByID(ctx, memo.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, memo.Content, got.Content)
	})

	t.Run("存在しないメモを更新する場合、メモ未検出エラーとなる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "memo-update-missing")
		repo := infrastructure.NewSQLiteMemoRepository(mgr.DB())
		memo := &domain.Memo{
			ID:        mustTestMemoID(t, 999),
			UserID:    mustTestUserID(t, 1),
			Content:   mustTestMemoContent(t, "new content"),
			CreatedAt: mustTestTime(t, "2026-07-09T12:00:00Z"),
		}

		// Act
		err := repo.Update(ctx, memo)

		// Assert
		require.ErrorIs(t, err, domain.ErrMemoNotFound)
	})

	t.Run("メモを削除する場合、ID検索で未検出となる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "memo-delete")
		userRepo := infrastructure.NewSQLiteUserRepository(mgr.DB())
		memoRepo := infrastructure.NewSQLiteMemoRepository(mgr.DB())
		user := &domain.User{Username: mustTestUsername(t, "alice"), PasswordHash: "hash-1"}
		require.NoError(t, userRepo.Create(ctx, user))
		memo := &domain.Memo{
			UserID:    user.ID,
			Content:   mustTestMemoContent(t, "remove me"),
			CreatedAt: mustTestTime(t, "2026-07-09T12:00:00Z"),
		}
		require.NoError(t, memoRepo.Create(ctx, memo))

		// Act
		err := memoRepo.Delete(ctx, memo.ID)

		// Assert
		require.NoError(t, err)
		got, findErr := memoRepo.FindByID(ctx, memo.ID)
		require.ErrorIs(t, findErr, domain.ErrMemoNotFound)
		require.Nil(t, got)
	})

	t.Run("存在しないメモを削除する場合、メモ未検出エラーとなる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "memo-delete-missing")
		repo := infrastructure.NewSQLiteMemoRepository(mgr.DB())

		// Act
		err := repo.Delete(ctx, mustTestMemoID(t, 999))

		// Assert
		require.ErrorIs(t, err, domain.ErrMemoNotFound)
	})

	t.Run("ユーザーIDでメモを検索する場合、対象ユーザーのメモのみ取得となる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "memo-by-user")
		userRepo := infrastructure.NewSQLiteUserRepository(mgr.DB())
		memoRepo := infrastructure.NewSQLiteMemoRepository(mgr.DB())

		alice := &domain.User{Username: mustTestUsername(t, "alice"), PasswordHash: "hash-1"}
		bob := &domain.User{Username: mustTestUsername(t, "bob"), PasswordHash: "hash-2"}
		require.NoError(t, userRepo.Create(ctx, alice))
		require.NoError(t, userRepo.Create(ctx, bob))

		first := &domain.Memo{UserID: alice.ID, Content: mustTestMemoContent(t, "one"), CreatedAt: mustTestTime(t, "2026-07-09T12:00:00Z")}
		second := &domain.Memo{UserID: alice.ID, Content: mustTestMemoContent(t, "two"), CreatedAt: mustTestTime(t, "2026-07-09T12:01:00Z")}
		other := &domain.Memo{UserID: bob.ID, Content: mustTestMemoContent(t, "three"), CreatedAt: mustTestTime(t, "2026-07-09T12:02:00Z")}
		require.NoError(t, memoRepo.Create(ctx, first))
		require.NoError(t, memoRepo.Create(ctx, second))
		require.NoError(t, memoRepo.Create(ctx, other))

		// Act
		got, err := memoRepo.FindByUserID(ctx, alice.ID)

		// Assert
		require.NoError(t, err)
		assert.Len(t, got, 2)
		for _, memo := range got {
			assert.Equal(t, alice.ID, memo.UserID)
		}
	})

	t.Run("ユーザーがメモを持たない場合、空の一覧となる", func(t *testing.T) {
		// Arrange
		mgr := newInitializedManager(t, "memo-by-user-empty")
		userRepo := infrastructure.NewSQLiteUserRepository(mgr.DB())
		memoRepo := infrastructure.NewSQLiteMemoRepository(mgr.DB())
		user := &domain.User{Username: mustTestUsername(t, "alice"), PasswordHash: "hash-1"}
		require.NoError(t, userRepo.Create(ctx, user))

		// Act
		got, err := memoRepo.FindByUserID(ctx, user.ID)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
