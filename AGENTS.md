# Repository Guidelines

## Project Structure & Modules
- `cmd/loginserver`, `cmd/gameserver`: entrypoints with `main.go` and `config.go` (env via `.env`/`.env.example`).
- `internal/loginserver|gameserver`: domain code split into `transport`, `handlers`, `usecase`, `repo`, `models`, `packets`, `registry`.
- `pkg/`: shared utilities (crypt, l2pkt).
- `loadtest/`: Node and Go clients for load testing.
- `l2jserver/`: protocol notes and reference materials.

## Architecture Overview
- LoginServer: Clean Architecture (handlers → usecase → repo → models). Packets: `packets/inclient`/`outclient` (client), `packets/ings`/`outgs` (GameServer).
- GameServer: Clean Architecture (handlers → usecase → repo → models). Packets: `packets/inclient`/`outclient` (client), `packets/inls`/`outls` (LoginServer). Uses PostgreSQL, XOR client crypto.

## Build, Test, Run
- `go mod tidy`: sync Go modules.
- `cd cmd/loginserver && go build -o loginserver .`: build LoginServer.
- `cd cmd/gameserver && go build -o gameserver .`: build GameServer.
- `go test ./...`: run all Go tests.
- Database: `docker-compose up -d postgres`, access via Adminer on port 8080.

## Coding Style & Naming
- Language: Go 1.22+; format with `gofmt` (tabs, standard imports). Run `go vet` before pushing.
- Packages: lowercase, no underscores; files lowercase with underscores only when meaningful.
- Exports: use Go CamelCase; keep `internal/*` not exported across modules.
- Logging: `zerolog`; prefer structured fields over printf.

## Testing Guidelines
- Framework: standard `testing` with table-driven tests.
- Locations: place `*_test.go` alongside code.
- Commands: `go test ./... -race -cover` for CI-like runs.

## Database & Migrations
- Both servers use PostgreSQL with pgx/v5 driver.
- LoginServer migrations: `internal/loginserver/schema/`
- GameServer migrations: `internal/gameserver/schema/` (6 files, 36+ indexes)
- Auto-run on server start; add new migrations rather than editing old ones.

## Commit & PR Guidelines
- Commits: concise, imperative messages; optional scope prefix (e.g., `LoginServer: fix session validation`).
- Checklist: `go test ./...` passes, builds succeed.

## Security & Configuration
- Never commit secrets; use `.env.example` as the template. Config is loaded via `envconfig`.
- LoginServer env: `cmd/loginserver/.env.example`
- GameServer env: `cmd/gameserver/.env.example`

## Protocol & Crypto Notes
- Java-compatible RSA (no padding) and Blowfish: static 22-byte key for LS<->GS; dynamic Blowfish per client; correct endianness and checksum required.

## Agent Boundaries

### l2j-java-expert Agent
**Scope**: Read-only analysis of Java L2J reference implementation in `l2jserver/` directory.
- Read and analyze Java L2J source code for protocol understanding
- Provide technical insights about packet structures and algorithms
- **NEVER modify files outside `l2jserver/`**
- **ONLY READ files in `l2jserver/` directory**

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->
