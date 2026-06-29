L2Go
====

L2Go is an open-source Lineage ][ server emulator written in golang.

## Quick Start

### Prerequisites
- Go 1.22+
- Docker (for PostgreSQL)

### 1. Start Database
```bash
docker-compose up -d postgres
```

### 2. Run Login Server
```bash
cd cmd/loginserver
cp .env.example .env
go build -o loginserver .
./loginserver
```

### 3. Run Game Server
```bash
cd cmd/gameserver
cp .env.example .env
go build -o gameserver .
./gameserver
```

Both servers auto-migrate the database on startup.

## Database Management

- **Web UI**: `docker-compose up -d adminer` → http://localhost:8080
- **Credentials**: postgres/postgres @ localhost:5432

See [DATABASE.md](DATABASE.md) for details.

## Current Status

**In development** — core world functionality working (auth, characters, movement, NPCs, targeting).

See [DOCKER_SETUP.md](DOCKER_SETUP.md) for Docker deployment.

Disclaimer
----

This work is for nonprofit educational purposes only. No copyright infringement is intended.

Using this work to run a private server, is at your own risk.

NCSOFT©, the Interlocking NC Logo, PLAYNC, Lineage, Team Lineage II, Lineage II, Goddess of Destruction, Valiance, Truly Free, L2 Store, L2 Galleria, NCoin, Path to Awakening, and all associated logos and designs are trademarks or registered trademarks of NCSOFT Corporation.

License
----

L2Go is under the GNU GPL v3.0 license.

Made with love by Frostwind <hi@frostwind.me>
