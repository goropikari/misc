package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
)

// SQLiteMemoRepository は SQLite を使ってメモ情報を保存・取得します。
type SQLiteMemoRepository struct {
	db *sql.DB
}

// NewSQLiteMemoRepository は db を使うリポジトリを作成します。
func NewSQLiteMemoRepository(db *sql.DB) *SQLiteMemoRepository {
	return &SQLiteMemoRepository{db: db}
}

// Create は新しいメモを挿入し、生成された ID を memo に反映します。
func (r *SQLiteMemoRepository) Create(ctx context.Context, memo *domain.Memo) error {
	if memo.CreatedAt.IsZero() {
		memo.CreatedAt = time.Now().UTC().Truncate(time.Second)
	}

	row := r.db.QueryRowContext(
		ctx,
		`INSERT INTO memos (user_id, content, created_at) VALUES (?, ?, ?) RETURNING id`,
		int64(memo.UserID),
		memo.Content.String(),
		memo.CreatedAt,
	)

	if err := row.Scan(&memo.ID); err != nil {
		return fmt.Errorf("failed to create memo: %w", err)
	}

	return nil
}

// FindByID は指定された ID のメモを返します。
func (r *SQLiteMemoRepository) FindByID(ctx context.Context, id domain.MemoID) (*domain.Memo, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, user_id, content, created_at FROM memos WHERE id = ?`,
		int64(id),
	)

	memo := &domain.Memo{}
	if err := row.Scan(&memo.ID, &memo.UserID, &memo.Content, &memo.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrMemoNotFound
		}
		return nil, fmt.Errorf("failed to find memo by id: %w", err)
	}

	return memo, nil
}

// Update は既存メモの保存済み内容を更新します。
func (r *SQLiteMemoRepository) Update(ctx context.Context, memo *domain.Memo) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE memos SET content = ?, created_at = ? WHERE id = ?`,
		memo.Content.String(),
		memo.CreatedAt,
		int64(memo.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to update memo: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to inspect updated memo count: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrMemoNotFound
	}

	return nil
}

// Delete は指定された ID のメモを削除します。
func (r *SQLiteMemoRepository) Delete(ctx context.Context, id domain.MemoID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM memos WHERE id = ?`, int64(id))
	if err != nil {
		return fmt.Errorf("failed to delete memo: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to inspect deleted memo count: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrMemoNotFound
	}

	return nil
}

// FindByUserID は指定されたユーザーが所有するメモをすべて返します。
func (r *SQLiteMemoRepository) FindByUserID(ctx context.Context, userID domain.UserID) ([]*domain.Memo, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, user_id, content, created_at FROM memos WHERE user_id = ? ORDER BY created_at ASC, id ASC`,
		int64(userID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list memos by user id: %w", err)
	}
	defer rows.Close()

	memos := make([]*domain.Memo, 0)
	for rows.Next() {
		memo := &domain.Memo{}
		if err := rows.Scan(&memo.ID, &memo.UserID, &memo.Content, &memo.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan memo: %w", err)
		}
		memos = append(memos, memo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate memos: %w", err)
	}

	return memos, nil
}

// SQLiteMemoRepository が domain.MemoRepository を満たすことを確認します。
var _ domain.MemoRepository = (*SQLiteMemoRepository)(nil)
