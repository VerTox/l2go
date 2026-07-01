# GameServer Development Guide

## Architecture

```
internal/gameserver/
├── handlers/client/   # Client packet handlers
│   ├── handler.go     # Main routing + handshake
│   ├── auth.go        # Client authentication
│   ├── character.go   # Character management
│   ├── world.go       # World entry + CharInfo + NPC spawn
│   ├── movement.go    # Movement + NPC visibility
│   ├── target.go      # Targeting NPCs/players
│   ├── logout.go      # Logout/restart
│   ├── actionuse.go   # Actions (Walk/Run etc.)
│   ├── inventory.go   # Items (UseItem, Unequip, ItemList)
│   └── multipacket.go # Multi-packet (0xD0) handlers
├── models/            # Domain entities
├── repo/              # PostgreSQL repositories
├── usecase/           # Business logic
├── transport/         # Client connections (XOR encryption)
├── registry/          # In-memory registries
│   ├── world.go       # WorldRegistry (players + NPCs + spatial index)
│   ├── npctemplates.go
│   ├── npcspawns.go
│   ├── itemtemplates.go
│   ├── objectid.go    # Atomic ObjectID generator (1,000,000+ for NPCs)
│   ├── connections.go
│   └── sessions.go
├── packets/           # Protocol packets
│   ├── inclient/      # Client -> GameServer
│   ├── outclient/     # GameServer -> Client
│   ├── inls/          # LoginServer -> GameServer
│   └── outls/         # GameServer -> LoginServer
└── schema/            # Database migrations (6 files, 36+ indexes)
```

## Packet Protocol Notes

- Packet opcodes verified against Java L2J reference (`l2jserver/`)
- Multi-packet (0xD0) uses 2-byte sub-opcodes
- UserInfo/CharInfo/NpcInfo structures match Java L2J byte-for-byte
- Use `pkg/l2pkt` Writer/Reader for all new packets

### Key Packet Pitfalls (lessons learned)
- SP field in UserInfo must be WriteD (4 bytes), NOT WriteQ (8 bytes) — causes alignment shift
- NpcInfo has exactly 8 speed fields, not 10
- NPC instances must have IsRunning=true for proper idle animation
- CharSelected packet structure differs significantly from UserInfo — verify against Java L2J
- StatusUpdate opcode is 0x18, NOT 0x0E — wrong opcode causes client freeze
- NPC interaction requires MoveToPawn (0x72) before NpcHtmlMessage — without it client blocks movement
- StatusUpdate uses WriteD (32-bit) for values — EXP (int64) gets truncated. Use UserInfo (WriteQ) for EXP updates
- MoveBackwardToLocation must NOT cancel auto-attack — client sends movement to approach target for melee
- CharInfo `InCombat` field must be read from PlayerWorldState, not hardcoded to 0

## NPC System

- Templates loaded from L2J XML datapack (`references/data/stats/npcs/`)
- Spawns from PostgreSQL (migration 006), auto-seeded from `references/data/spawnlist.sql` (~38K entries)
- Dynamic visibility: KnownNPCs per player, NpcInfo on enter range, DeleteObject on leave (2500 units)
- ObjectID for NPC instances starts at 1,000,000 (atomic counter)

## Current TODOs

**Phase 8 — NPC Interaction & Combat:**
1. ~~NPC dialogue system (NpcHtmlMessage packets, HTML windows)~~ DONE
2. Trading with NPC merchants
3. Gatekeeper teleportation
4. ~~Basic attack mechanics (click-to-attack, damage calculation)~~ DONE
5. ~~HP management, death/respawn~~ DONE
6. NPC AI (aggro, patrol, return-to-spawn)

**Phase 9+ — Future:**
- Full ExBasicActionList (189 actions, currently only Walk/Run)
- Skill system (casting, effects, cooldowns)
- Quest system
- Party system
- PvP

## Combat System

- **Game loop** (`gameloop/`) owns all mutable NPC state and combat logic in a single goroutine
- **Auto-attack**: `CmdAttackRequest` sets ATTACK intention → server-side move-to-target (tick interpolates position) → `NextAttackEvent` combat-heartbeat checks reach against the server position → `HitEvent` (damage at mid-swing) → next swing cycle
- **Approach**: out-of-reach `NextAttackEvent`/`InteractApproachEvent` restart server-side movement (`startMoveToTarget`) and re-check on the next heartbeat (~400ms); the server position (not stale client packets) drives arrival. A ground move cancels the intention (stops chasing).
- **Movement does NOT cancel attack**: `MoveBackwardToLocation` no longer sends `CmdCancelAttack` — the client sends movement packets to approach the target for melee
- **EXP/SP rewards**: Proportional distribution via hate list, level penalty for large level gaps, server rate multipliers. Uses UserInfo (64-bit) instead of StatusUpdate (32-bit truncation) for EXP updates
- **Combat stance**: `PlayerWorldState.InCombat` flag propagated to CharInfo/UserInfo packets. 15s timeout after last attack via `CombatStanceTimeoutEvent`
- **Logout in combat**: Blocked with SystemMessage(1116) + ActionFailed. Allowed after combat stance expires
- **NPC retaliation**: NPCs auto-attack back when hit (via hate list)
- **Death/respawn**: Die packet → corpse decay (7s) → respawn (60s) with new ObjectID

## Known Limitations

- Movement speed computed per-character (base×DEX from race/class); item/buff modifiers are a no-op hook (`applyMoveSpeedBonus`) pending item-stats/skill systems
- Item type classification approximate (no full item template DB yet)
- Multi-packet handler covers only a few sub-opcodes of 50+
- No collision detection