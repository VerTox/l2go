package models

import "testing"

func TestNormalizeAccountName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"already lowercase", "vertox", "vertox"},
		{"mixed case", "VerTox", "vertox"},
		{"all uppercase", "ADMIN", "admin"},
		{"surrounding whitespace", "  VerTox  ", "vertox"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeAccountName(tt.in); got != tt.want {
				t.Errorf("NormalizeAccountName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeAccountName_Idempotent(t *testing.T) {
	once := NormalizeAccountName("VerTox")
	twice := NormalizeAccountName(once)
	if once != twice {
		t.Errorf("not idempotent: %q -> %q", once, twice)
	}
}
