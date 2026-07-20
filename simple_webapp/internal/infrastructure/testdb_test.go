package infrastructure_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-txdb"
	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
	"github.com/goropikari/study_memo/simple_webapp/internal/infrastructure"
	"github.com/stretchr/testify/require"
)

var testDBRegistry = struct {
	sync.Mutex
	states map[string]*testDBState
}{
	states: map[string]*testDBState{},
}

func init() {
	sql.Register("memdb", testDriver{})
}

type testDBState struct {
	mu sync.Mutex

	users           map[int64]*domain.User
	usersByUsername map[string]int64
	memos           map[int64]*domain.Memo

	nextUserID int64
	nextMemoID int64

	usersTableCreated bool
	memosTableCreated bool
	foreignKeysOn     bool
}

func (s *testDBState) clone() *testDBState {
	s.mu.Lock()
	defer s.mu.Unlock()

	clone := newTestState()
	clone.nextUserID = s.nextUserID
	clone.nextMemoID = s.nextMemoID
	clone.usersTableCreated = s.usersTableCreated
	clone.memosTableCreated = s.memosTableCreated
	clone.foreignKeysOn = s.foreignKeysOn

	for id, user := range s.users {
		copyUser := *user
		clone.users[id] = &copyUser
		clone.usersByUsername[user.Username.String()] = id
	}
	for id, memo := range s.memos {
		copyMemo := *memo
		clone.memos[id] = &copyMemo
	}

	return clone
}

func (s *testDBState) restore(snapshot *testDBState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.users = map[int64]*domain.User{}
	s.usersByUsername = map[string]int64{}
	s.memos = map[int64]*domain.Memo{}

	s.nextUserID = snapshot.nextUserID
	s.nextMemoID = snapshot.nextMemoID
	s.usersTableCreated = snapshot.usersTableCreated
	s.memosTableCreated = snapshot.memosTableCreated
	s.foreignKeysOn = snapshot.foreignKeysOn

	for id, user := range snapshot.users {
		copyUser := *user
		s.users[id] = &copyUser
		s.usersByUsername[user.Username.String()] = id
	}
	for id, memo := range snapshot.memos {
		copyMemo := *memo
		s.memos[id] = &copyMemo
	}
}

func newTestState() *testDBState {
	return &testDBState{
		users:           map[int64]*domain.User{},
		usersByUsername: map[string]int64{},
		memos:           map[int64]*domain.Memo{},
	}
}

func getTestState(name string) *testDBState {
	testDBRegistry.Lock()
	defer testDBRegistry.Unlock()

	state, ok := testDBRegistry.states[name]
	if !ok {
		state = newTestState()
		testDBRegistry.states[name] = state
	}

	return state
}

type testDriver struct{}

func (testDriver) Open(name string) (driver.Conn, error) {
	return &testConn{state: getTestState(name)}, nil
}

type testConn struct {
	state *testDBState
}

func (c *testConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported in test driver")
}

func (c *testConn) Close() error {
	return nil
}

func (c *testConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *testConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return &testTx{
		state:    c.state,
		snapshot: c.state.clone(),
	}, nil
}

func (c *testConn) Ping(context.Context) error {
	return nil
}

func (c *testConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	_ = ctx

	rows, lastID, err := c.exec(query, args)
	if err != nil {
		return nil, err
	}

	return testResult{lastInsertID: lastID, rowsAffected: rows}, nil
}

func (c *testConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	_ = ctx

	return c.query(query, args)
}

type testTx struct {
	state    *testDBState
	snapshot *testDBState
}

func (tx *testTx) Commit() error {
	return nil
}

func (tx *testTx) Rollback() error {
	tx.state.restore(tx.snapshot)
	return nil
}

func (c *testConn) exec(query string, args []driver.NamedValue) (int64, int64, error) {
	normalized := normalizeSQL(query)

	switch {
	case strings.HasPrefix(normalized, "pragma foreign_keys = on"):
		c.state.mu.Lock()
		c.state.foreignKeysOn = true
		c.state.mu.Unlock()
		return 0, 0, nil
	case strings.HasPrefix(normalized, "create table if not exists users"):
		c.state.mu.Lock()
		c.state.usersTableCreated = true
		c.state.mu.Unlock()
		return 0, 0, nil
	case strings.HasPrefix(normalized, "create table if not exists memos"):
		c.state.mu.Lock()
		c.state.memosTableCreated = true
		c.state.mu.Unlock()
		return 0, 0, nil
	case strings.HasPrefix(normalized, "insert into users"):
		id, err := c.insertUser(args)
		if err != nil {
			return 0, 0, err
		}
		return 1, id, nil
	case strings.HasPrefix(normalized, "insert into memos"):
		id, err := c.insertMemo(args)
		if err != nil {
			return 0, 0, err
		}
		return 1, id, nil
	case strings.HasPrefix(normalized, "update memos"):
		return c.updateMemo(args)
	case strings.HasPrefix(normalized, "delete from memos"):
		return c.deleteMemo(args)
	default:
		return 0, 0, fmt.Errorf("unsupported exec query: %s", query)
	}
}

func (c *testConn) query(query string, args []driver.NamedValue) (driver.Rows, error) {
	normalized := normalizeSQL(query)

	switch {
	case strings.HasPrefix(normalized, "select name from sqlite_master"):
		if len(args) == 0 {
			return &testRows{columns: []string{"name"}}, nil
		}
		tableName, err := namedValueString(args, 0)
		if err != nil {
			return nil, err
		}
		c.state.mu.Lock()
		exists := (tableName == "users" && c.state.usersTableCreated) || (tableName == "memos" && c.state.memosTableCreated)
		c.state.mu.Unlock()
		if !exists {
			return &testRows{columns: []string{"name"}}, nil
		}
		return &testRows{
			columns: []string{"name"},
			values:  [][]driver.Value{{tableName}},
		}, nil
	case strings.HasPrefix(normalized, "insert into users") && strings.Contains(normalized, "returning id"):
		id, err := c.insertUser(args)
		if err != nil {
			return nil, err
		}
		return &testRows{
			columns: []string{"id"},
			values:  [][]driver.Value{{id}},
		}, nil
	case strings.HasPrefix(normalized, "insert into memos") && strings.Contains(normalized, "returning id"):
		id, err := c.insertMemo(args)
		if err != nil {
			return nil, err
		}
		return &testRows{
			columns: []string{"id"},
			values:  [][]driver.Value{{id}},
		}, nil
	case strings.HasPrefix(normalized, "select id, username, password_hash from users where username ="):
		username, err := namedValueString(args, 0)
		if err != nil {
			return nil, err
		}
		user, ok := c.findUserByUsername(username)
		if !ok {
			return &testRows{columns: []string{"id", "username", "password_hash"}}, nil
		}
		return &testRows{
			columns: []string{"id", "username", "password_hash"},
			values: [][]driver.Value{{
				int64(user.ID),
				user.Username.String(),
				user.PasswordHash,
			}},
		}, nil
	case strings.HasPrefix(normalized, "select id, username, password_hash from users where id ="):
		id, err := namedValueInt64(args, 0)
		if err != nil {
			return nil, err
		}
		user, ok := c.findUserByID(id)
		if !ok {
			return &testRows{columns: []string{"id", "username", "password_hash"}}, nil
		}
		return &testRows{
			columns: []string{"id", "username", "password_hash"},
			values: [][]driver.Value{{
				int64(user.ID),
				user.Username.String(),
				user.PasswordHash,
			}},
		}, nil
	case strings.HasPrefix(normalized, "select id, user_id, content, created_at from memos where id ="):
		id, err := namedValueInt64(args, 0)
		if err != nil {
			return nil, err
		}
		memo, ok := c.findMemoByID(id)
		if !ok {
			return &testRows{columns: []string{"id", "user_id", "content", "created_at"}}, nil
		}
		return &testRows{
			columns: []string{"id", "user_id", "content", "created_at"},
			values: [][]driver.Value{{
				int64(memo.ID),
				int64(memo.UserID),
				memo.Content.String(),
				memo.CreatedAt,
			}},
		}, nil
	case strings.HasPrefix(normalized, "select id, user_id, content, created_at from memos where user_id ="):
		userID, err := namedValueInt64(args, 0)
		if err != nil {
			return nil, err
		}
		rows := c.findMemosByUserID(userID)
		return &testRows{
			columns: []string{"id", "user_id", "content", "created_at"},
			values:  rows,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported query: %s", query)
	}
}

func (c *testConn) insertUser(args []driver.NamedValue) (int64, error) {
	username, err := namedValueString(args, 0)
	if err != nil {
		return 0, err
	}
	passwordHash, err := namedValueString(args, 1)
	if err != nil {
		return 0, err
	}

	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	if _, exists := c.state.usersByUsername[username]; exists {
		return 0, errors.New("UNIQUE constraint failed: users.username")
	}

	c.state.nextUserID++
	id := c.state.nextUserID
	user := &domain.User{
		ID:           domain.UserID(id),
		Username:     domain.Username(username),
		PasswordHash: passwordHash,
	}
	c.state.users[id] = user
	c.state.usersByUsername[username] = id

	return id, nil
}

func (c *testConn) insertMemo(args []driver.NamedValue) (int64, error) {
	userID, err := namedValueInt64(args, 0)
	if err != nil {
		return 0, err
	}
	content, err := namedValueString(args, 1)
	if err != nil {
		return 0, err
	}

	var createdAt time.Time
	if len(args) > 2 {
		createdAt, err = namedValueTime(args, 2)
		if err != nil {
			return 0, err
		}
	}

	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	if c.state.foreignKeysOn {
		if _, exists := c.state.users[userID]; !exists {
			return 0, errors.New("FOREIGN KEY constraint failed")
		}
	}

	c.state.nextMemoID++
	id := c.state.nextMemoID
	if createdAt.IsZero() {
		createdAt = time.Now().UTC().Truncate(time.Second)
	}

	memo := &domain.Memo{
		ID:        domain.MemoID(id),
		UserID:    domain.UserID(userID),
		Content:   domain.MemoContent(content),
		CreatedAt: createdAt,
	}
	c.state.memos[id] = memo

	return id, nil
}

func (c *testConn) updateMemo(args []driver.NamedValue) (int64, int64, error) {
	if len(args) < 2 {
		return 0, 0, fmt.Errorf("update memo requires arguments")
	}

	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	id, err := namedValueInt64(args, len(args)-1)
	if err != nil {
		return 0, 0, err
	}
	memo, exists := c.state.memos[id]
	if !exists {
		return 0, 0, nil
	}

	content, err := namedValueString(args, 0)
	if err != nil {
		return 0, 0, err
	}
	memo.Content = domain.MemoContent(content)

	if len(args) > 2 {
		createdAt, err := namedValueTime(args, 1)
		if err != nil {
			return 0, 0, err
		}
		memo.CreatedAt = createdAt
	}

	return 1, 0, nil
}

func (c *testConn) deleteMemo(args []driver.NamedValue) (int64, int64, error) {
	id, err := namedValueInt64(args, 0)
	if err != nil {
		return 0, 0, err
	}

	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	if _, exists := c.state.memos[id]; !exists {
		return 0, 0, nil
	}

	delete(c.state.memos, id)
	return 1, 0, nil
}

func (c *testConn) findUserByUsername(username string) (*domain.User, bool) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	id, exists := c.state.usersByUsername[username]
	if !exists {
		return nil, false
	}

	user, ok := c.state.users[id]
	if !ok {
		return nil, false
	}

	copy := *user
	return &copy, true
}

func (c *testConn) findUserByID(id int64) (*domain.User, bool) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	user, exists := c.state.users[id]
	if !exists {
		return nil, false
	}

	copy := *user
	return &copy, true
}

func (c *testConn) findMemoByID(id int64) (*domain.Memo, bool) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	memo, exists := c.state.memos[id]
	if !exists {
		return nil, false
	}

	copy := *memo
	return &copy, true
}

func (c *testConn) findMemosByUserID(userID int64) [][]driver.Value {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	var memos []*domain.Memo
	for _, memo := range c.state.memos {
		if int64(memo.UserID) == userID {
			copy := *memo
			memos = append(memos, &copy)
		}
	}

	sortMemos(memos)

	rows := make([][]driver.Value, 0, len(memos))
	for _, memo := range memos {
		rows = append(rows, []driver.Value{
			int64(memo.ID),
			int64(memo.UserID),
			memo.Content.String(),
			memo.CreatedAt,
		})
	}

	return rows
}

func sortMemos(memos []*domain.Memo) {
	for i := 0; i < len(memos); i++ {
		for j := i + 1; j < len(memos); j++ {
			if memos[j].CreatedAt.Before(memos[i].CreatedAt) ||
				(memos[j].CreatedAt.Equal(memos[i].CreatedAt) && memos[j].ID < memos[i].ID) {
				memos[i], memos[j] = memos[j], memos[i]
			}
		}
	}
}

type testRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *testRows) Columns() []string {
	return r.columns
}

func (r *testRows) Close() error {
	return nil
}

func (r *testRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}

	copy(dest, r.values[r.index])
	r.index++
	return nil
}

type testResult struct {
	lastInsertID int64
	rowsAffected int64
}

func (r testResult) LastInsertId() (int64, error) {
	return r.lastInsertID, nil
}

func (r testResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

func normalizeSQL(query string) string {
	return strings.Join(strings.Fields(strings.ToLower(query)), " ")
}

func namedValueString(args []driver.NamedValue, index int) (string, error) {
	if index >= len(args) {
		return "", fmt.Errorf("missing argument %d", index)
	}

	switch v := args[index].Value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("argument %d is not a string: %T", index, args[index].Value)
	}
}

func namedValueInt64(args []driver.NamedValue, index int) (int64, error) {
	if index >= len(args) {
		return 0, fmt.Errorf("missing argument %d", index)
	}

	switch v := args[index].Value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case domain.UserID:
		return int64(v), nil
	case domain.MemoID:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("argument %d is not an integer: %T", index, args[index].Value)
	}
}

func namedValueTime(args []driver.NamedValue, index int) (time.Time, error) {
	if index >= len(args) {
		return time.Time{}, fmt.Errorf("missing argument %d", index)
	}

	switch v := args[index].Value.(type) {
	case time.Time:
		return v, nil
	case string:
		return time.Parse(time.RFC3339Nano, v)
	default:
		return time.Time{}, fmt.Errorf("argument %d is not a time: %T", index, args[index].Value)
	}
}

func newTestDB(t *testing.T, name string) *sql.DB {
	t.Helper()

	db := sql.OpenDB(txdb.New("memdb", name))
	t.Cleanup(func() {
		_ = db.Close()
	})

	require.NoError(t, db.Ping())

	return db
}

func newDirectDB(t *testing.T, name string) *sql.DB {
	t.Helper()

	db, err := sql.Open("memdb", name)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	require.NoError(t, db.Ping())

	return db
}

func newInitializedManager(t *testing.T, name string) *infrastructure.DBManager {
	t.Helper()

	mgr := infrastructure.NewDBManagerWithDB(newTestDB(t, name))
	require.NoError(t, mgr.InitSchema())
	return mgr
}

func tableExists(t *testing.T, db *sql.DB, tableName string) bool {
	t.Helper()

	row := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, tableName)
	var name string
	if err := row.Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		require.NoError(t, err)
	}
	return name == tableName
}
