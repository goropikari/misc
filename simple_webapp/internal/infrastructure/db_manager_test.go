package infrastructure_test

import (
	"context"
	"testing"
	"time"

	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
	"github.com/goropikari/study_memo/simple_webapp/internal/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBManager_InitSchemaCreatesTables(t *testing.T) {
	t.Run("スキーマを初期化する場合、usersテーブルとmemosテーブルが作成済みとなる", func(t *testing.T) {
		// Arrange
		mgr := infrastructure.NewDBManagerWithDB(newTestDB(t, "schema-create"))

		// Act
		err := mgr.InitSchema()

		// Assert
		require.NoError(t, err)
		assert.True(t, tableExists(t, mgr.DB(), "users"))
		assert.True(t, tableExists(t, mgr.DB(), "memos"))
	})
}

func TestDBManager_CloseClosesDatabase(t *testing.T) {
	t.Run("DBを閉じる場合、以後のPingはエラーとなる", func(t *testing.T) {
		// Arrange
		mgr := infrastructure.NewDBManagerWithDB(newTestDB(t, "close-db"))

		// Act
		err := mgr.Close()

		// Assert
		require.NoError(t, err)
		assert.Error(t, mgr.DB().Ping())
	})
}

func TestDBManager_PersistsDataAcrossReconnect(t *testing.T) {
	t.Run("同じDBに再接続する場合、保存済みデータが取得可能となる", func(t *testing.T) {
		// Arrange
		const dbName = "persist-db"
		ctx := context.Background()
		firstMgr := infrastructure.NewDBManagerWithDB(newDirectDB(t, dbName))
		require.NoError(t, firstMgr.InitSchema())
		userRepo := infrastructure.NewSQLiteUserRepository(firstMgr.DB())
		memoRepo := infrastructure.NewSQLiteMemoRepository(firstMgr.DB())

		user := &domain.User{Username: mustTestUsername(t, "alice"), PasswordHash: "hashed_password_123"}
		memo := &domain.Memo{Content: mustTestMemoContent(t, "persistent memo"), CreatedAt: mustTestTime(t, "2026-07-09T12:00:00Z")}

		require.NoError(t, userRepo.Create(ctx, user))
		memo.UserID = user.ID
		require.NoError(t, memoRepo.Create(ctx, memo))

		// Act
		require.NoError(t, firstMgr.Close())
		secondMgr := infrastructure.NewDBManagerWithDB(newDirectDB(t, dbName))
		require.NoError(t, secondMgr.InitSchema())

		reopenedUserRepo := infrastructure.NewSQLiteUserRepository(secondMgr.DB())
		reopenedMemoRepo := infrastructure.NewSQLiteMemoRepository(secondMgr.DB())
		gotUser, userErr := reopenedUserRepo.FindByID(ctx, user.ID)
		gotMemo, memoErr := reopenedMemoRepo.FindByID(ctx, memo.ID)

		// Assert
		require.NoError(t, userErr)
		require.NotNil(t, gotUser)
		assert.Equal(t, user.Username, gotUser.Username)
		assert.Equal(t, user.PasswordHash, gotUser.PasswordHash)
		require.NoError(t, memoErr)
		require.NotNil(t, gotMemo)
		assert.Equal(t, memo.Content, gotMemo.Content)
		assert.Equal(t, memo.UserID, gotMemo.UserID)
		assert.False(t, gotMemo.CreatedAt.IsZero())
	})
}

func mustTestUserID(t *testing.T, value int64) domain.UserID {
	t.Helper()

	userID, err := domain.NewUserID(value)
	require.NoError(t, err)
	return userID
}

func mustTestMemoID(t *testing.T, value int64) domain.MemoID {
	t.Helper()

	memoID, err := domain.NewMemoID(value)
	require.NoError(t, err)
	return memoID
}

func mustTestUsername(t *testing.T, value string) domain.Username {
	t.Helper()

	username, err := domain.NewUsername(value)
	require.NoError(t, err)
	return username
}

func mustTestMemoContent(t *testing.T, value string) domain.MemoContent {
	t.Helper()

	content, err := domain.NewMemoContent(value)
	require.NoError(t, err)
	return content
}

func mustTestTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)
	return parsed
}
