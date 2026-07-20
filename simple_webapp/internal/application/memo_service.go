package application

import (
	"context"
	"fmt"

	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
)

// MemoService はメモ管理のビジネスロジックと認可を処理します。
type MemoService struct {
	memoRepo domain.MemoRepository
}

// NewMemoService は MemoService の新しいインスタンスを作成します。
func NewMemoService(repo domain.MemoRepository) *MemoService {
	return &MemoService{memoRepo: repo}
}

// CreateMemo は指定されたユーザーのために新しいメモを作成します。
func (s *MemoService) CreateMemo(ctx context.Context, userID domain.UserID, content domain.MemoContent) (domain.MemoID, error) {
	memo := &domain.Memo{
		UserID:  userID,
		Content: content,
	}

	if err := s.memoRepo.Create(ctx, memo); err != nil {
		return 0, fmt.Errorf("failed to create memo: %w", err)
	}

	return memo.ID, nil
}

// GetMemo はメモを取得し、要求したユーザーが所有者であることを確認します。
func (s *MemoService) GetMemo(ctx context.Context, userID domain.UserID, memoID domain.MemoID) (*domain.Memo, error) {
	memo, err := s.memoRepo.FindByID(ctx, memoID)
	if err != nil {
		return nil, err
	}

	if memo.UserID != userID {
		return nil, domain.ErrUnauthorized
	}

	return memo, nil
}

// UpdateMemo は要求したユーザーが所有者の場合にのみ、メモの内容を更新します。
func (s *MemoService) UpdateMemo(ctx context.Context, userID domain.UserID, memoID domain.MemoID, content domain.MemoContent) error {
	memo, err := s.memoRepo.FindByID(ctx, memoID)
	if err != nil {
		return err
	}

	if memo.UserID != userID {
		return domain.ErrUnauthorized
	}

	memo.Content = content
	if err := s.memoRepo.Update(ctx, memo); err != nil {
		return fmt.Errorf("failed to update memo: %w", err)
	}

	return nil
}

// DeleteMemo は要求したユーザーが所有者の場合にのみ、メモを削除します。
func (s *MemoService) DeleteMemo(ctx context.Context, userID domain.UserID, memoID domain.MemoID) error {
	memo, err := s.memoRepo.FindByID(ctx, memoID)
	if err != nil {
		return err
	}

	if memo.UserID != userID {
		return domain.ErrUnauthorized
	}

	if err := s.memoRepo.Delete(ctx, memoID); err != nil {
		return fmt.Errorf("failed to delete memo: %w", err)
	}

	return nil
}

// ListMemos は指定されたユーザーに属するすべてのメモを取得します。
func (s *MemoService) ListMemos(ctx context.Context, userID domain.UserID) ([]*domain.Memo, error) {
	memos, err := s.memoRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list memos: %w", err)
	}
	return memos, nil
}
