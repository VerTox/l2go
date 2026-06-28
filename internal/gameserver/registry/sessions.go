package registry

import "sync"

// Sessions tracks active client sessions and online count.
type Sessions struct {
    mu       sync.RWMutex
    byAcct   map[string]int64 // account -> connID
    byConnID map[int64]string // connID  -> account
}

func NewSessions() *Sessions {
    return &Sessions{byAcct: make(map[string]int64), byConnID: make(map[int64]string)}
}

func (s *Sessions) Attach(connID int64, account string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.byAcct[account] = connID
    s.byConnID[connID] = account
}

func (s *Sessions) Detach(connID int64) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if acc, ok := s.byConnID[connID]; ok {
        delete(s.byConnID, connID)
        delete(s.byAcct, acc)
    }
}

func (s *Sessions) OnlineCount() int {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return len(s.byConnID)
}

