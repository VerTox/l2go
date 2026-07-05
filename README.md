# L2Go

[![CI](https://github.com/VerTox/l2go/actions/workflows/ci.yml/badge.svg)](https://github.com/VerTox/l2go/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/VerTox/l2go/branch/master/graph/badge.svg)](https://codecov.io/gh/VerTox/l2go)
[![Go version](https://img.shields.io/github/go-mod/go-version/VerTox/l2go)](go.mod)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

**An open-source Lineage II server emulator written in Go.**

L2Go implements a full LoginServer + GameServer stack with byte-for-byte protocol
compatibility, targeting the **High Five (Chronicle 5)** client. Built with Clean
Architecture, PostgreSQL persistence, and a single-goroutine authoritative game loop.

---

## Features

| Working today | |
|---|---|
| 🔐 **Auth** | Full client ↔ LoginServer ↔ GameServer flow (Blowfish/RSA/XOR) |
| 🧍 **Characters** | Creation, selection, deletion, persistence |
| 🌍 **World** | Entry, visibility, movement (run/walk), broadcasting |
| 🐺 **NPCs** | ~39K spawns from the L2J datapack, dynamic visibility, dialogue |
| ⚔️ **Combat** | Auto-attack, hit/miss/crit, retaliation, death/respawn, EXP/SP |
| ✨ **Skills** | Casting, effects, buffs/toggles (HoT/DoT), passives, reuse |
| 🎒 **Items** | Inventory, equipment, potions, soul/spirit shots, enchant, recipes |
| ❤️ **Vitals** | HP/MP/CP regeneration, level-up |

_Not yet implemented:_ NPC trading, gatekeeper teleport, quests, parties, PvP.

---

## Quick Start

### Option A — Full stack via Docker (recommended)

Brings up PostgreSQL, LoginServer, GameServer, Adminer, Prometheus and Grafana.
The game datapack is baked into the image — no extra setup.

```bash
make up          # or: docker compose up -d
```

Connect a High Five client to `127.0.0.1:2106`.

### Option B — Native (for development)

Requires **Go 1.23+** and a PostgreSQL instance (`make db-up` starts one in Docker).

```bash
make db-up                 # start PostgreSQL
cp cmd/loginserver/.env.example cmd/loginserver/.env
cp cmd/gameserver/.env.example  cmd/gameserver/.env
make run-loginserver       # terminal 1
make run-gameserver        # terminal 2
```

Both servers auto-migrate the database on startup.

---

## Ports

| Service | Port | Purpose |
|---|---|---|
| LoginServer | `2106` | Game client login |
| LoginServer | `9014` | GameServer ↔ LoginServer |
| GameServer | `7777` | Game client world |
| GameServer | `2112` | Prometheus metrics |
| Adminer | `8080` | Database web UI |
| Grafana | `3000` | Live-ops dashboards |

---

## Project Layout

```
cmd/            loginserver, gameserver, stressbot, seedbots entrypoints
internal/       loginserver + gameserver (handlers → usecase → repo → models)
pkg/            shared crypto (Blowfish/XOR) and packet reader/writer
datapack/       runtime L2J data (NPC/item/skill XML, spawnlist, …) — shipped in-tree
```

PostgreSQL migrations live under `internal/*/schema/` and run automatically on startup.

Architecture follows Clean Architecture with dependency injection and a
thread-safe world registry. The game loop owns all mutable world state on a
single goroutine; handlers relay commands to it.

---

## Common Commands

Run `make help` for the full list. Highlights:

```bash
make up / down          # full docker stack up / down
make restart-game       # rebuild + restart just the GameServer
make logs-game          # tail GameServer logs
make test               # go test ./...
make stress N=500       # ramp 500 load-test bots into the world
```

---

## Disclaimer

This work is for nonprofit educational purposes only. No copyright infringement is
intended. Using this work to run a private server is at your own risk.

NCSOFT©, the Interlocking NC Logo, PLAYNC, Lineage, Team Lineage II, Lineage II and
all associated logos and designs are trademarks or registered trademarks of NCSOFT
Corporation.

## License

L2Go is licensed under the **GNU GPL v3.0**.

Originally created with love by Frostwind &lt;hi@frostwind.me&gt;.
