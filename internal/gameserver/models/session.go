package models

import (
	"sync"
	"time"
)

// PlayerSession represents an active player session in the GameServer
type PlayerSession struct {
	// Session identification
	SessionID   string    `json:"session_id"`
	AccountName string    `json:"account_name"`
	CreatedAt   time.Time `json:"created_at"`
	LastActive  time.Time `json:"last_active"`

	// Authentication data from LoginServer
	LoginKey1 uint32 `json:"login_key1"`
	LoginKey2 uint32 `json:"login_key2"`
	PlayKey1  uint32 `json:"play_key1"`
	PlayKey2  uint32 `json:"play_key2"`

	// Connection information
	ClientIP   string `json:"client_ip"`
	ClientPort int    `json:"client_port"`

	// Character information (set when character is selected)
	CharacterID   *int32     `json:"character_id,omitempty"`
	CharacterName string     `json:"character_name,omitempty"`
	Character     *Character `json:"character,omitempty"`

	// Session state
	State         SessionState `json:"state"`
	Authenticated bool         `json:"authenticated"`

	// Connection metadata
	Protocol        int    `json:"protocol"`
	ClientVersion   string `json:"client_version,omitempty"`
	HardwareID      string `json:"hardware_id,omitempty"`
	LastPacketTime  time.Time
	PacketCount     int64
	BytesSent       int64
	BytesReceived   int64

	// Thread safety
	mutex sync.RWMutex
}

// SessionState represents the current state of a player session
type SessionState int

const (
	StateConnected    SessionState = 0 // Initial connection
	StateAuthenticated SessionState = 1 // Authenticated with LoginServer
	StateCharSelect   SessionState = 2 // In character selection screen
	StateInGame       SessionState = 3 // Playing with selected character
	StateDisconnected SessionState = 4 // Disconnected/expired
)

// SessionKey represents session authentication keys
type SessionKey struct {
	PlayKey1  uint32 `json:"play_key1"`
	PlayKey2  uint32 `json:"play_key2"`
	LoginKey1 uint32 `json:"login_key1"`
	LoginKey2 uint32 `json:"login_key2"`
}

// NewPlayerSession creates a new player session
func NewPlayerSession(sessionID, accountName, clientIP string, clientPort int) *PlayerSession {
	now := time.Now()
	return &PlayerSession{
		SessionID:      sessionID,
		AccountName:    accountName,
		CreatedAt:      now,
		LastActive:     now,
		LastPacketTime: now,
		ClientIP:       clientIP,
		ClientPort:     clientPort,
		State:          StateConnected,
		Authenticated:  false,
	}
}

// SetSessionKeys sets the authentication keys from LoginServer
func (s *PlayerSession) SetSessionKeys(keys SessionKey) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.LoginKey1 = keys.LoginKey1
	s.LoginKey2 = keys.LoginKey2
	s.PlayKey1 = keys.PlayKey1
	s.PlayKey2 = keys.PlayKey2
}

// GetSessionKeys returns current session keys
func (s *PlayerSession) GetSessionKeys() SessionKey {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return SessionKey{
		LoginKey1: s.LoginKey1,
		LoginKey2: s.LoginKey2,
		PlayKey1:  s.PlayKey1,
		PlayKey2:  s.PlayKey2,
	}
}

// SetAuthenticated marks session as authenticated
func (s *PlayerSession) SetAuthenticated(authenticated bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.Authenticated = authenticated
	if authenticated {
		s.State = StateAuthenticated
	}
	s.LastActive = time.Now()
}

// IsAuthenticated returns authentication status
func (s *PlayerSession) IsAuthenticated() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.Authenticated
}

// SetState updates session state
func (s *PlayerSession) SetState(state SessionState) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.State = state
	s.LastActive = time.Now()
}

// GetState returns current session state
func (s *PlayerSession) GetState() SessionState {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.State
}

// SelectCharacter sets the active character for this session
func (s *PlayerSession) SelectCharacter(character *Character) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.CharacterID = &character.ID
	s.CharacterName = character.Name
	s.Character = character
	s.State = StateCharSelect
	s.LastActive = time.Now()
}

// EnterGame moves session to in-game state
func (s *PlayerSession) EnterGame() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.State = StateInGame
	s.LastActive = time.Now()
}

// GetCharacter returns the selected character (thread-safe)
func (s *PlayerSession) GetCharacter() *Character {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.Character
}

// HasCharacter returns true if a character is selected
func (s *PlayerSession) HasCharacter() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.Character != nil
}

// IsInGame returns true if session is actively playing
func (s *PlayerSession) IsInGame() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.State == StateInGame && s.Character != nil
}

// UpdateActivity updates last activity timestamp
func (s *PlayerSession) UpdateActivity() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.LastActive = time.Now()
	s.LastPacketTime = time.Now()
	s.PacketCount++
}

// UpdateTraffic updates network traffic counters
func (s *PlayerSession) UpdateTraffic(sent, received int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.BytesSent += sent
	s.BytesReceived += received
}

// GetSessionDuration returns how long the session has been active
func (s *PlayerSession) GetSessionDuration() time.Duration {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return time.Since(s.CreatedAt)
}

// GetIdleTime returns how long since last activity
func (s *PlayerSession) GetIdleTime() time.Duration {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return time.Since(s.LastActive)
}

// IsExpired returns true if session is expired
func (s *PlayerSession) IsExpired(timeout time.Duration) bool {
	return s.GetIdleTime() > timeout
}

// Disconnect marks session as disconnected
func (s *PlayerSession) Disconnect() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.State = StateDisconnected
	s.LastActive = time.Now()
}

// IsDisconnected returns true if session is disconnected
func (s *PlayerSession) IsDisconnected() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.State == StateDisconnected
}

// GetStateName returns human-readable state name
func (s *PlayerSession) GetStateName() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	switch s.State {
	case StateConnected:
		return "Connected"
	case StateAuthenticated:
		return "Authenticated"
	case StateCharSelect:
		return "Character Selection"
	case StateInGame:
		return "In Game"
	case StateDisconnected:
		return "Disconnected"
	default:
		return "Unknown"
	}
}

// GetSessionInfo returns session information for monitoring/debugging
func (s *PlayerSession) GetSessionInfo() SessionInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return SessionInfo{
		SessionID:       s.SessionID,
		AccountName:     s.AccountName,
		CharacterName:   s.CharacterName,
		ClientIP:        s.ClientIP,
		State:           s.GetStateName(),
		Duration:        s.GetSessionDuration(),
		IdleTime:        s.GetIdleTime(),
		PacketCount:     s.PacketCount,
		BytesSent:       s.BytesSent,
		BytesReceived:   s.BytesReceived,
		Authenticated:   s.Authenticated,
	}
}

// SessionInfo represents session information for monitoring
type SessionInfo struct {
	SessionID     string        `json:"session_id"`
	AccountName   string        `json:"account_name"`
	CharacterName string        `json:"character_name,omitempty"`
	ClientIP      string        `json:"client_ip"`
	State         string        `json:"state"`
	Duration      time.Duration `json:"duration"`
	IdleTime      time.Duration `json:"idle_time"`
	PacketCount   int64         `json:"packet_count"`
	BytesSent     int64         `json:"bytes_sent"`
	BytesReceived int64         `json:"bytes_received"`
	Authenticated bool          `json:"authenticated"`
}

// SessionRegistry manages active player sessions
type SessionRegistry struct {
	sessions map[string]*PlayerSession
	byAccount map[string]*PlayerSession
	mutex    sync.RWMutex
}

// NewSessionRegistry creates a new session registry
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions:  make(map[string]*PlayerSession),
		byAccount: make(map[string]*PlayerSession),
	}
}

// AddSession adds a new session to the registry
func (r *SessionRegistry) AddSession(session *PlayerSession) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.sessions[session.SessionID] = session
	r.byAccount[session.AccountName] = session
}

// GetSession retrieves a session by session ID
func (r *SessionRegistry) GetSession(sessionID string) (*PlayerSession, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	session, exists := r.sessions[sessionID]
	return session, exists
}

// GetSessionByAccount retrieves a session by account name
func (r *SessionRegistry) GetSessionByAccount(accountName string) (*PlayerSession, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	session, exists := r.byAccount[accountName]
	return session, exists
}

// RemoveSession removes a session from the registry
func (r *SessionRegistry) RemoveSession(sessionID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	if session, exists := r.sessions[sessionID]; exists {
		delete(r.sessions, sessionID)
		delete(r.byAccount, session.AccountName)
	}
}

// RemoveSessionByAccount removes a session by account name
func (r *SessionRegistry) RemoveSessionByAccount(accountName string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	if session, exists := r.byAccount[accountName]; exists {
		delete(r.sessions, session.SessionID)
		delete(r.byAccount, accountName)
	}
}

// GetAllSessions returns all active sessions
func (r *SessionRegistry) GetAllSessions() []*PlayerSession {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	sessions := make([]*PlayerSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// GetSessionCount returns total number of active sessions
func (r *SessionRegistry) GetSessionCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	return len(r.sessions)
}

// GetInGameCount returns number of sessions currently in-game
func (r *SessionRegistry) GetInGameCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	count := 0
	for _, session := range r.sessions {
		if session.IsInGame() {
			count++
		}
	}
	return count
}

// CleanupExpiredSessions removes expired sessions
func (r *SessionRegistry) CleanupExpiredSessions(timeout time.Duration) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	var expiredSessions []string
	for sessionID, session := range r.sessions {
		if session.IsExpired(timeout) {
			expiredSessions = append(expiredSessions, sessionID)
		}
	}
	
	for _, sessionID := range expiredSessions {
		if session, exists := r.sessions[sessionID]; exists {
			delete(r.sessions, sessionID)
			delete(r.byAccount, session.AccountName)
		}
	}
	
	return len(expiredSessions)
}

// IsAccountOnline checks if an account is currently online
func (r *SessionRegistry) IsAccountOnline(accountName string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	session, exists := r.byAccount[accountName]
	return exists && !session.IsDisconnected()
}

// GetSessionStats returns registry statistics
func (r *SessionRegistry) GetSessionStats() SessionStats {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	stats := SessionStats{
		TotalSessions: len(r.sessions),
	}
	
	for _, session := range r.sessions {
		switch session.GetState() {
		case StateConnected:
			stats.Connected++
		case StateAuthenticated:
			stats.Authenticated++
		case StateCharSelect:
			stats.CharSelect++
		case StateInGame:
			stats.InGame++
		case StateDisconnected:
			stats.Disconnected++
		}
	}
	
	return stats
}

// SessionStats represents session registry statistics
type SessionStats struct {
	TotalSessions int `json:"total_sessions"`
	Connected     int `json:"connected"`
	Authenticated int `json:"authenticated"`
	CharSelect    int `json:"char_select"`
	InGame        int `json:"in_game"`
	Disconnected  int `json:"disconnected"`
}

// Session-related errors
var (
	ErrSessionNotFound    = &SessionError{"session not found"}
	ErrSessionExpired     = &SessionError{"session expired"}
	ErrAccountOnline      = &SessionError{"account already online"}
	ErrInvalidSessionKey  = &SessionError{"invalid session key"}
	ErrNotAuthenticated   = &SessionError{"session not authenticated"}
	ErrNoCharacterSelected = &SessionError{"no character selected"}
)

// SessionError represents session-related errors
type SessionError struct {
	msg string
}

func (e *SessionError) Error() string {
	return e.msg
}