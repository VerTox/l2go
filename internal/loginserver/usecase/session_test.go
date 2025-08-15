package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/loginserver/models"
)

func TestSessionUseCase(t *testing.T) {
	ctx := context.Background()
	sessionUC := NewSessionUseCase(1 * time.Minute) // 1 minute expiration for testing

	// Test creating session key
	sessionKey, err := sessionUC.CreateSessionKey(ctx, "testuser", 1, 0x12345678, 0x87654321)
	if err != nil {
		t.Fatalf("Failed to create session key: %v", err)
	}

	if sessionKey.Account != "testuser" {
		t.Errorf("Expected account 'testuser', got '%s'", sessionKey.Account)
	}

	if sessionKey.ServerID != 1 {
		t.Errorf("Expected server ID 1, got %d", sessionKey.ServerID)
	}

	if sessionKey.LoginKey1 != 0x12345678 {
		t.Errorf("Expected LoginKey1 0x12345678, got %08X", sessionKey.LoginKey1)
	}

	if sessionKey.LoginKey2 != 0x87654321 {
		t.Errorf("Expected LoginKey2 0x87654321, got %08X", sessionKey.LoginKey2)
	}

	// Test validating correct session key
	requestedKey := models.SessionKey{
		LoginKey1: sessionKey.LoginKey1,
		LoginKey2: sessionKey.LoginKey2,
		PlayKey1:  sessionKey.PlayKey1,
		PlayKey2:  sessionKey.PlayKey2,
	}

	valid, err := sessionUC.ValidateSessionKey(ctx, "testuser", requestedKey)
	if err != nil {
		t.Fatalf("Failed to validate session key: %v", err)
	}

	if !valid {
		t.Error("Expected session key to be valid, but validation failed")
	}

	// Test validating wrong session key
	wrongKey := models.SessionKey{
		LoginKey1: 0x11111111,
		LoginKey2: 0x22222222,
		PlayKey1:  0x33333333,
		PlayKey2:  0x44444444,
	}

	valid, err = sessionUC.ValidateSessionKey(ctx, "testuser", wrongKey)
	if err != nil {
		t.Fatalf("Failed to validate wrong session key: %v", err)
	}

	if valid {
		t.Error("Expected wrong session key to be invalid, but validation passed")
	}

	// Test validating session key for non-existent user
	valid, err = sessionUC.ValidateSessionKey(ctx, "nonexistent", requestedKey)
	if err != nil {
		t.Fatalf("Failed to validate session key for non-existent user: %v", err)
	}

	if valid {
		t.Error("Expected session key validation to fail for non-existent user")
	}

	// Test getting session key
	retrievedKey, err := sessionUC.GetSessionKey(ctx, "testuser")
	if err != nil {
		t.Fatalf("Failed to get session key: %v", err)
	}

	if !retrievedKey.Equals(*sessionKey) {
		t.Error("Retrieved session key does not match original")
	}

	// Test removing session key
	err = sessionUC.RemoveSessionKey(ctx, "testuser")
	if err != nil {
		t.Fatalf("Failed to remove session key: %v", err)
	}

	// Test getting removed session key should fail
	_, err = sessionUC.GetSessionKey(ctx, "testuser")
	if err == nil {
		t.Error("Expected error when getting removed session key")
	}

	// Test session count
	if sessionUC.GetSessionCount() != 0 {
		t.Errorf("Expected session count 0, got %d", sessionUC.GetSessionCount())
	}
}

func TestSessionExpiration(t *testing.T) {
	ctx := context.Background()
	sessionUC := NewSessionUseCase(100 * time.Millisecond) // Very short expiration

	// Create session key
	sessionKey, err := sessionUC.CreateSessionKey(ctx, "testuser", 1, 0x12345678, 0x87654321)
	if err != nil {
		t.Fatalf("Failed to create session key: %v", err)
	}

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Test that expired session key is invalid
	requestedKey := models.SessionKey{
		LoginKey1: sessionKey.LoginKey1,
		LoginKey2: sessionKey.LoginKey2,
		PlayKey1:  sessionKey.PlayKey1,
		PlayKey2:  sessionKey.PlayKey2,
	}

	valid, err := sessionUC.ValidateSessionKey(ctx, "testuser", requestedKey)
	if err != nil {
		t.Fatalf("Failed to validate expired session key: %v", err)
	}

	if valid {
		t.Error("Expected expired session key to be invalid")
	}

	// Test cleanup
	sessionUC.CreateSessionKey(ctx, "user1", 1, 0x11111111, 0x22222222)
	sessionUC.CreateSessionKey(ctx, "user2", 1, 0x33333333, 0x44444444)

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	expiredCount := sessionUC.CleanupExpiredSessions(ctx)
	if expiredCount != 2 {
		t.Errorf("Expected 2 expired sessions cleaned up, got %d", expiredCount)
	}

	if sessionUC.GetSessionCount() != 0 {
		t.Errorf("Expected 0 sessions after cleanup, got %d", sessionUC.GetSessionCount())
	}
}

func TestSessionReplacement(t *testing.T) {
	ctx := context.Background()
	sessionUC := NewSessionUseCase(1 * time.Minute)

	// Create first session
	session1, err := sessionUC.CreateSessionKey(ctx, "testuser", 1, 0x11111111, 0x22222222)
	if err != nil {
		t.Fatalf("Failed to create first session key: %v", err)
	}

	// Create second session for same user (should replace first)
	session2, err := sessionUC.CreateSessionKey(ctx, "testuser", 2, 0x33333333, 0x44444444)
	if err != nil {
		t.Fatalf("Failed to create second session key: %v", err)
	}

	// Should have only 1 session
	if sessionUC.GetSessionCount() != 1 {
		t.Errorf("Expected 1 session after replacement, got %d", sessionUC.GetSessionCount())
	}

	// First session should be invalid
	requestedKey1 := models.SessionKey{
		LoginKey1: session1.LoginKey1,
		LoginKey2: session1.LoginKey2,
		PlayKey1:  session1.PlayKey1,
		PlayKey2:  session1.PlayKey2,
	}

	valid, _ := sessionUC.ValidateSessionKey(ctx, "testuser", requestedKey1)
	if valid {
		t.Error("Expected first session to be invalid after replacement")
	}

	// Second session should be valid
	requestedKey2 := models.SessionKey{
		LoginKey1: session2.LoginKey1,
		LoginKey2: session2.LoginKey2,
		PlayKey1:  session2.PlayKey1,
		PlayKey2:  session2.PlayKey2,
	}

	valid, _ = sessionUC.ValidateSessionKey(ctx, "testuser", requestedKey2)
	if !valid {
		t.Error("Expected second session to be valid")
	}
}
