package infrastructure

import (
	"context"
	"sync"

	"github.com/goropikari/study_memo/simple_webapp/internal/domain"
)

// MemorySessionStore はプロセス内メモリでセッションを保持します。
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]domain.UserID
}

// NewMemorySessionStore は空のメモリセッションストアを作成します。
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{sessions: map[string]domain.UserID{}}
}

// Save はセッション ID とユーザー ID の対応を保存します。
func (s *MemorySessionStore) Save(_ context.Context, sessionID string, userID domain.UserID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[sessionID] = userID
	return nil
}

// FindUserID はセッション ID に対応するユーザー ID を返します。
func (s *MemorySessionStore) FindUserID(_ context.Context, sessionID string) (domain.UserID, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, ok := s.sessions[sessionID]
	if !ok {
		return 0, domain.ErrUnauthorized
	}
	return userID, nil
}

// Delete はセッション ID を削除します。
func (s *MemorySessionStore) Delete(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
	return nil
}
