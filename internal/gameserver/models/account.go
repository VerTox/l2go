package models

import "strings"

// NormalizeAccountName returns the canonical form of an account name used
// throughout the game server: lowercase and trimmed. Account names are treated
// case-insensitively (matching L2J, where the login is lowercased), so this must
// be applied at every boundary that ingests an account name (client auth,
// character creation) to keep DB rows, in-memory comparisons and registry map keys
// all in one canonical case.
func NormalizeAccountName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
