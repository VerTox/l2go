# L2Go GameServer

L2Go GameServer is a Lineage II game server implementation written in Go, designed to work with the L2Go LoginServer.

## Configuration

Copy `.env.example` to `.env` and modify the values as needed.

### Environment Variables

#### GameServer Settings
- `GAME_SERVER_ID` - Unique server ID (default: 1)
- `GAME_SERVER_NAME` - Server name displayed to players (default: "L2Go Bartz")
- `GAME_SERVER_PORT` - Port for game client connections (default: 7777)
- `GAME_SERVER_MAX_PLAYERS` - Maximum concurrent players (default: 1000)
- `GAME_SERVER_TYPE` - Server type: 1=Normal, 2=Relax, 4=Test (default: 1)

#### LoginServer Connection
- `LOGIN_SERVER_HOST` - LoginServer hostname (default: "127.0.0.1")
- `LOGIN_SERVER_PORT` - LoginServer GameServer port (default: "9014")

#### Network
- `GAME_HOST` - Host for game client connections (default: "127.0.0.1")
- `GAME_PORT` - Port for game client connections (default: "7777")

#### Database (PostgreSQL)
- `POSTGRES_HOST` - PostgreSQL hostname (default: "127.0.0.1")
- `POSTGRES_PORT` - PostgreSQL port (default: "5432")
- `POSTGRES_USERNAME` - PostgreSQL username (default: "l2go")
- `POSTGRES_PASSWORD` - PostgreSQL password (default: "l2go")
- `POSTGRES_DATABASE` - PostgreSQL database name (default: "l2go_gameserver")

## Building and Running

### Prerequisites
1. Go 1.22+
2. Running L2Go LoginServer
3. PostgreSQL database

### Build & Run
```bash
cp .env.example .env
# Edit .env with your settings
go build -o gameserver .
./gameserver
```

## Integration with LoginServer

The GameServer automatically:
1. Connects to the LoginServer on startup
2. Authenticates using AuthRequest packet
3. Exchanges encryption keys (RSA + Blowfish)
4. Sends heartbeat every 30 seconds with ServerStatus
5. Handles player authentication requests from LoginServer
6. Reports player login/logout events
