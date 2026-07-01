-- Migration 007: Normalize account_name to lowercase
-- Account names are treated case-insensitively across the game server (matching
-- L2J, where the login is lowercased). Characters were historically stored with
-- the original login case, while login->game packets carry the lowercased login,
-- so mixed-case rows break account_name lookups. Canonicalize existing rows to
-- lowercase so the (now normalized-at-ingress) queries match. (l2go-xhp)

UPDATE characters
SET account_name = LOWER(account_name)
WHERE account_name <> LOWER(account_name);
