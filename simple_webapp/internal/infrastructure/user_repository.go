package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
)

// SQLiteUserRepository は SQLite を使ってユーザー情報を保存・取得します。
type SQLiteUserRepository struct {
	db *sql.DB
}

// NewSQLiteUserRepository は db を使うリポジトリを作成します。
func NewSQLiteUserRepository(db *sql.DB) *SQLiteUserRepository {
	return &SQLiteUserRepository{db: db}
}

// Create は新しいユーザーを挿入し、生成された ID を user に反映します。
func (r *SQLiteUserRepository) Create(ctx context.Context, user *domain.User) error {
	row := r.db.QueryRowContext(
		ctx,
		`INSERT INTO users (username, password_hash) VALUES (?, ?) RETURNING id`,
		user.Username.String(),
		user.PasswordHash,
	)

	if err := row.Scan(&user.ID); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.username") {
			return domain.ErrUserAlreadyExists
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// FindByUsername は指定されたユーザー名のユーザーを返します。
func (r *SQLiteUserRepository) FindByUsername(ctx context.Context, username domain.Username) (*domain.User, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, username, password_hash FROM users WHERE username = ?`,
		username.String(),
	)

	user := &domain.User{}
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user by username: %w", err)
	}

	return user, nil
}

// FindByID は指定された ID のユーザーを返します。
func (r *SQLiteUserRepository) FindByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, username, password_hash FROM users WHERE id = ?`,
		int64(id),
	)

	user := &domain.User{}
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user by id: %w", err)
	}

	return user, nil
}

// SQLiteUserRepository が domain.UserRepository を満たすことを確認します。
var _ domain.UserRepository = (*SQLiteUserRepository)(nil)
