package inls

import (
	"bytes"
	"testing"
)

// utf16le кодирует строку в UTF-16LE с нулевым терминатором (как протокол L2).
func utf16le(s string) []byte {
	out := make([]byte, 0, len(s)*2+2)
	for _, r := range s {
		out = append(out, byte(r), byte(r>>8))
	}
	out = append(out, 0x00, 0x00)
	return out
}

func TestAuthResponse(t *testing.T) {
	data := append([]byte{0x05}, utf16le("BartzServer")...)
	p := NewAuthResponse(data)
	if p.GetServerID() != 5 {
		t.Errorf("ServerID = %d, want 5", p.GetServerID())
	}
	if p.GetServerName() != "BartzServer" {
		t.Errorf("ServerName = %q, want %q", p.GetServerName(), "BartzServer")
	}
}

func TestChangePasswordResponse(t *testing.T) {
	data := append(utf16le("myaccount"), 0x01)
	p := NewChangePasswordResponse(data)
	if p.GetAccount() != "myaccount" {
		t.Errorf("Account = %q, want %q", p.GetAccount(), "myaccount")
	}
	if !p.HasChanged() {
		t.Errorf("HasChanged = false, want true")
	}
}

func TestInitLS(t *testing.T) {
	rsaKey := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE}
	// keySize (uint32 LE) + key bytes
	data := append([]byte{0x05, 0x00, 0x00, 0x00}, rsaKey...)
	p := NewInitLS(data)
	if !bytes.Equal(p.GetRSAKey(), rsaKey) {
		t.Errorf("RSAKey = %x, want %x", p.GetRSAKey(), rsaKey)
	}
}

func TestKickPlayer(t *testing.T) {
	p := NewKickPlayer(utf16le("baduser"))
	if p.GetAccount() != "baduser" {
		t.Errorf("Account = %q, want %q", p.GetAccount(), "baduser")
	}
}

func TestLoginServerFail(t *testing.T) {
	p := NewLoginServerFail(utf16le("rejected"))
	if p.GetReason() != "rejected" {
		t.Errorf("Reason = %q, want %q", p.GetReason(), "rejected")
	}
}

func TestPlayerAuthResponse(t *testing.T) {
	data := append(utf16le("acc"), 0x01)
	p := NewPlayerAuthResponse(data)
	if p.GetAccount() != "acc" {
		t.Errorf("Account = %q, want %q", p.GetAccount(), "acc")
	}
	if !p.IsAuthed() {
		t.Errorf("IsAuthed = false, want true")
	}

	data2 := append(utf16le("acc"), 0x00)
	if NewPlayerAuthResponse(data2).IsAuthed() {
		t.Errorf("IsAuthed = true, want false")
	}
}

func TestRequestCharacters(t *testing.T) {
	p := NewRequestCharacters(utf16le("someacc"))
	if p.GetAccount() != "someacc" {
		t.Errorf("Account = %q, want %q", p.GetAccount(), "someacc")
	}
}
