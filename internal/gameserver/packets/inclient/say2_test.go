package inclient

import "testing"

// dword кодирует int32 в little-endian (4 байта).
func dword(v int32) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

func TestSay2All(t *testing.T) {
	// text="hi", type=ALL(0), no target
	payload := append(utf16le("hi"), dword(0)...)
	p := NewSay2(payload)
	if p.Text != "hi" {
		t.Errorf("Text = %q, want %q", p.Text, "hi")
	}
	if p.Type != 0 {
		t.Errorf("Type = %d, want 0", p.Type)
	}
	if p.Target != "" {
		t.Errorf("Target = %q, want empty", p.Target)
	}
}

func TestSay2Tell(t *testing.T) {
	// text="hello", type=TELL(2), target="Bob"
	payload := utf16le("hello")
	payload = append(payload, dword(2)...)
	payload = append(payload, utf16le("Bob")...)
	p := NewSay2(payload)
	if p.Text != "hello" {
		t.Errorf("Text = %q, want %q", p.Text, "hello")
	}
	if p.Type != 2 {
		t.Errorf("Type = %d, want 2", p.Type)
	}
	if p.Target != "Bob" {
		t.Errorf("Target = %q, want %q", p.Target, "Bob")
	}
}

func TestSay2NonTellHasNoTarget(t *testing.T) {
	// SHOUT(1) must NOT read a target string (there is none on the wire).
	payload := append(utf16le("!wts"), dword(1)...)
	p := NewSay2(payload)
	if p.Type != 1 {
		t.Errorf("Type = %d, want 1", p.Type)
	}
	if p.Target != "" {
		t.Errorf("Target = %q, want empty for non-TELL", p.Target)
	}
}
