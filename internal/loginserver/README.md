# LoginServer Internal Architecture

This package implements the new LoginServer architecture using Clean Architecture principles.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                          Handlers                               │
│  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────────┐ │
│  │ AuthGameGuard   │  │ RequestAuthLogin │  │ RequestServerList│ │
│  │                 │  │                 │  │                  │ │
│  └─────────────────┘  └─────────────────┘  └──────────────────┘ │
│  ┌─────────────────┐  ┌─────────────────┐                       │
│  │ RequestPlay     │  │ RequestServerlist │                     │
│  │                 │  │                 │                       │
│  └─────────────────┘  └─────────────────┘                       │
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Use Cases                               │
│  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────────┐ │
│  │ ClientUseCase   │  │ GameServerUC    │  │ SessionUseCase   │ │
│  │ - Auth          │  │ - Registry      │  │ - Key Management │ │
│  │ - ServerLogin   │  │ - Filtering     │  │ - Validation     │ │
│  └─────────────────┘  └─────────────────┘  └──────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Infrastructure                               │
│  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────────┐ │
│  │ Repository      │  │ GameServerReg   │  │ Transport        │ │
│  │ - PostgreSQL    │  │ - In-Memory     │  │ - TCP Client     │ │
│  │ - Accounts      │  │ - Thread-Safe   │  │ - Encryption     │ │
│  └─────────────────┘  └─────────────────┘  └──────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Package Structure

```
internal/loginserver/
├── handlers/
│   ├── client/              # Client packet handlers
│   │   ├── authgameguard.go
│   │   ├── requestauthlogin.go
│   │   ├── requestserverlist.go
│   │   └── requestserverlogin.go
│   └── gameserver/          # GameServer packet handlers
│       ├── handler.go           # Main handler structure
│       ├── authrequest.go       # GameServer authentication
│       ├── blowfishkey.go       # Encryption key exchange
│       ├── playerauth.go        # Player authentication
│       ├── playeringame.go      # Player login notifications
│       ├── playerlogout.go      # Player logout handling
│       ├── playertracert.go     # Network traceroute data
│       ├── replycharacters.go   # Character count responses
│       └── serverstatus.go      # Server status updates
├── models/                  # Domain models
│   ├── account.go
│   ├── gameserver.go
│   └── session.go
├── packets/                 # Protocol implementation
│   ├── inclient/           # Client → Server packets
│   ├── outclient/          # Server → Client packets
│   ├── ings/               # GameServer → LoginServer packets
│   ├── outgs/              # LoginServer → GameServer packets
│   ├── helpers.go
│   └── packets.go
├── registry/               # In-memory registries
│   ├── gameserver.go
│   └── charactercounts.go
├── repo/                   # Data persistence
│   ├── interfaces.go
│   └── postgres.go
├── transport/              # Network layer
│   └── client.go
├── usecase/                # Business logic
│   ├── client.go
│   ├── gameserver.go
│   ├── gameservercomm.go
│   └── session.go
├── events/                 # Event-driven architecture
│   └── events.go
├── schema/                 # Database migrations
│   ├── schema.go
│   └── *.sql
└── service.go              # Service composition
```

## Key Features

### ✅ Authentication Flow
1. **AuthGameGuard**: Session ID validation
2. **RequestAuthLogin**: Username/password authentication
3. **Account Storage**: Stores authenticated account in client
4. **Access Control**: Sets client access level for filtering

### ✅ Server List Management
1. **GameServer Registry**: Thread-safe in-memory storage
2. **Server Filtering**: Hide GM-only/test servers from normal players
3. **Server Status**: Online/Offline/GM-Only/Test server states
4. **Demo Servers**: Auto-registration of test servers on startup

### ✅ Session Key Management  
1. **Key Generation**: Cryptographically secure session keys
2. **Key Validation**: Validates session keys for GameServer authentication
3. **Expiration**: Automatic cleanup of expired sessions
4. **Thread Safety**: Concurrent-safe session operations

### ✅ Server Connection Flow
1. **RequestServerLogin**: Validate server access and create session
2. **Server Validation**: Check server exists and is accessible
3. **Access Control**: Test servers require admin access
4. **PlayOk Response**: Returns session keys for GameServer handoff

## Usage Example

```go
// Initialize LoginServer
loginParams := loginserver.Params{
    DB:             db,
    LoginHost:      "127.0.0.1",
    LoginPort:      "2106", 
    GameServerPort: "9014",
}

service := loginserver.New(loginParams)

// Run server
ctx := context.Background()
if err := service.Run(ctx); err != nil {
    log.Fatal("Failed to run login server:", err)
}
```

## Protocol Flow

```
Client                 LoginServer              GameServer
  │                        │                        │
  │ ── AuthGameGuard ───→   │                        │
  │ ←─── GGAuth ─────────   │                        │
  │                        │                        │
  │ ── RequestAuthLogin ─→  │                        │
  │ ←─── LoginOk ────────   │                        │
  │                        │                        │
  │ ── RequestServerList ─→ │                        │
  │ ←─── ServerList ──────  │                        │
  │                        │                        │
  │ ── RequestServerLogin ─→│                        │
  │ ←─── PlayOk ──────────  │                        │
  │                        │                        │
  │ ── Connect to GameServer ──────────────────────→ │
                           │ ← PlayerAuthRequest ──  │
                           │ ── PlayerAuthResponse → │
```

## Testing

```bash
# Run all tests
go test ./internal/loginserver/...

# Run specific test suites
go test ./internal/loginserver/registry -v
go test ./internal/loginserver/usecase -v
```

