package models

import "time"

// SessionKey represents the session key structure for player authentication
type SessionKey struct {
	LoginKey1 uint32    `json:"login_key1"`
	LoginKey2 uint32    `json:"login_key2"`
	PlayKey1  uint32    `json:"play_key1"`
	PlayKey2  uint32    `json:"play_key2"`
	Account   string    `json:"account"`
	ServerID  int       `json:"server_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Equals compares two session keys for equality
func (sk *SessionKey) Equals(other SessionKey) bool {
	return sk.LoginKey1 == other.LoginKey1 &&
		sk.LoginKey2 == other.LoginKey2 &&
		sk.PlayKey1 == other.PlayKey1 &&
		sk.PlayKey2 == other.PlayKey2
}

// IsExpired checks if the session key is expired (older than specified duration)
func (sk *SessionKey) IsExpired(maxAge time.Duration) bool {
	return time.Since(sk.CreatedAt) > maxAge
}
