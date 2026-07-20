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
)

func TestMemoService(t *testing.T) {
	ctx := context.Background()

	t.Run("メモを作成する場合、作成成功となる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockMemoRepository(ctrl)
		service := application.NewMemoService(repo)
		userID := mustUserID(t, 1)
		content := mustMemoContent(t, "study memo")

		repo.EXPECT().
			Create(ctx, gomock.Any()).
			Do(func(_ context.Context, memo *domain.Memo) {
				memo.ID = 100
			}).
			Return(nil).
			Times(1)

		// Act
		memoID, err := service.CreateMemo(ctx, userID, content)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, domain.MemoID(100), memoID)
	})

	t.Run("ユーザーがメモの所有者の場合、メモ取得成功となる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockMemoRepository(ctrl)
		service := application.NewMemoService(repo)
		userID := mustUserID(t, 1)
		memoID := mustMemoID(t, 100)
		memo := &domain.Memo{ID: memoID, UserID: userID, Content: mustMemoContent(t, "owned memo")}

		repo.EXPECT().
			FindByID(ctx, memoID).
			Return(memo, nil).
			Times(1)

		// Act
		got, err := service.GetMemo(ctx, userID, memoID)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, memoID, got.ID)
	})

	t.Run("ユーザーがメモの所有者ではない場合、権限エラーとなる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockMemoRepository(ctrl)
		service := application.NewMemoService(repo)
		ownerID := mustUserID(t, 1)
		otherUserID := mustUserID(t, 2)
		memoID := mustMemoID(t, 100)
		memo := &domain.Memo{ID: memoID, UserID: ownerID, Content: mustMemoContent(t, "owned memo")}

		repo.EXPECT().
			FindByID(ctx, memoID).
			Return(memo, nil).
			Times(1)

		// Act
		got, err := service.GetMemo(ctx, otherUserID, memoID)

		// Assert
		require.ErrorIs(t, err, domain.ErrUnauthorized)
		require.Nil(t, got)
	})

	t.Run("メモが存在しない場合、メモ未検出エラーとなる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockMemoRepository(ctrl)
		service := application.NewMemoService(repo)
		userID := mustUserID(t, 1)
		memoID := mustMemoID(t, 404)

		repo.EXPECT().
			FindByID(ctx, memoID).
			Return(nil, domain.ErrMemoNotFound).
			Times(1)

		// Act
		_, err := service.GetMemo(ctx, userID, memoID)

		// Assert
		require.ErrorIs(t, err, domain.ErrMemoNotFound)
	})

	t.Run("ユーザーがメモの所有者の場合、メモ更新成功となる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockMemoRepository(ctrl)
		service := application.NewMemoService(repo)
		userID := mustUserID(t, 1)
		memoID := mustMemoID(t, 100)
		memo := &domain.Memo{ID: memoID, UserID: userID, Content: mustMemoContent(t, "before")}
		updatedContent := mustMemoContent(t, "after")

		repo.EXPECT().
			FindByID(ctx, memoID).
			Return(memo, nil).
			Times(1)
		repo.EXPECT().
			Update(ctx, gomock.Any()).
			Return(nil).
			Times(1)

		// Act
		err := service.UpdateMemo(ctx, userID, memoID, updatedContent)

		// Assert
		require.NoError(t, err)
	})

	t.Run("ユーザーがメモの所有者ではない場合、メモ更新は権限エラーとなる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockMemoRepository(ctrl)
		service := application.NewMemoService(repo)
		ownerID := mustUserID(t, 1)
		otherUserID := mustUserID(t, 2)
		memoID := mustMemoID(t, 100)
		memo := &domain.Memo{ID: memoID, UserID: ownerID, Content: mustMemoContent(t, "before")}
		updatedContent := mustMemoContent(t, "after")

		repo.EXPECT().
			FindByID(ctx, memoID).
			Return(memo, nil).
			Times(1)

		// Act
		err := service.UpdateMemo(ctx, otherUserID, memoID, updatedContent)

		// Assert
		require.ErrorIs(t, err, domain.ErrUnauthorized)
	})

	t.Run("ユーザーがメモの所有者の場合、メモ削除成功となる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockMemoRepository(ctrl)
		service := application.NewMemoService(repo)
		userID := mustUserID(t, 1)
		memoID := mustMemoID(t, 100)
		memo := &domain.Memo{ID: memoID, UserID: userID, Content: mustMemoContent(t, "owned memo")}

		repo.EXPECT().
			FindByID(ctx, memoID).
			Return(memo, nil).
			Times(1)
		repo.EXPECT().
			Delete(ctx, memoID).
			Return(nil).
			Times(1)

		// Act
		err := service.DeleteMemo(ctx, userID, memoID)

		// Assert
		require.NoError(t, err)
	})

	t.Run("ユーザーがメモの所有者ではない場合、メモ削除は権限エラーとなる", func(t *testing.T) {
		// Arrange
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := mock.NewMockMemoRepository(ctrl)
		service := application.NewMemoService(repo)
		ownerID := mustUserID(t, 1)
		otherUserID := mustUserID(t, 2)
		memoID := mustMemoID(t, 100)
		memo := &domain.Memo{ID: memoID, UserID: ownerID, Content: mustMemoContent(t, "owned memo")}

		repo.EXPECT().
			FindByID(ctx, memoID).
			Return(memo, nil).
			Times(1)

		// Act
		err := service.DeleteMemo(ctx, otherUserID, memoID)

		// Assert
		require.ErrorIs(t, err, domain.ErrUnauthorized)
	})
}

func mustUserID(t *testing.T, value int64) domain.UserID {
	t.Helper()
	userID, err := domain.NewUserID(value)
	require.NoError(t, err)
	return userID
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
