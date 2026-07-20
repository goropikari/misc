package infrastructure

import (
	"database/sql"
	"fmt"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// DBManager はリポジトリが利用する SQLite データベース接続を管理します。
type DBManager struct {
	db *sql.DB
}

// NewDBManager は databasePath の SQLite データベース用に DBManager を作成します。
func NewDBManager(databasePath string) (*DBManager, error) {
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	mgr := &DBManager{db: db}
	if err := mgr.InitSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return mgr, nil
}

// NewDBManagerWithDB は既存のデータベース接続から DBManager を作成します。
func NewDBManagerWithDB(db *sql.DB) *DBManager {
	return &DBManager{db: db}
}

// DB は管理対象のデータベース接続を返します。
func (m *DBManager) DB() *sql.DB {
	return m.db
}

// InitSchema は、アプリケーションに必要なテーブルが無ければ作成します。
func (m *DBManager) InitSchema() error {
	if _, err := m.db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	usersSchema := `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL
)`
	if _, err := m.db.Exec(usersSchema); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	memosSchema := `
CREATE TABLE IF NOT EXISTS memos (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	content TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
)`
	if _, err := m.db.Exec(memosSchema); err != nil {
		return fmt.Errorf("failed to create memos table: %w", err)
	}

	return nil
}

// Close は管理対象のデータベース接続を閉じます。
func (m *DBManager) Close() error {
	if m.db == nil {
		return nil
	}
	return m.db.Close()
}
