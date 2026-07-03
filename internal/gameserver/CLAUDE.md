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
- **No player auto-retaliation**: a player hit by a mob enters combat stance but does NOT auto-attack back — matches retail HF. L2J's `L2PlayerAI` doesn't override `onEvtAttacked`; the base `L2CharacterAI.onEvtAttacked` only calls `clientStartAutoAttack()` (stance/AutoAttackStart), never `doAttack`. Players attack only on explicit request. (l2go-i75)
- **Death/respawn**: Die packet → corpse decay (7s) → respawn (60s) with new ObjectID

## Item Use & Handlers (ItemHandler dispatch — epic l2go-irn, DONE)

- **Dispatch**: `usecase/inventory.go` `UseItem` forks equip-vs-etc. Non-equip items route through `useNonEquipItem`, which looks up a handler by `template.Handler` (string) in `ItemHandlerRegistry`. **No handler = silent no-op, not an error** (mirrors L2J). Handlers implement `ItemHandler{ UseItem(ctx, ItemUseContext) (consumed bool, err error) }`.
- **Registration**: all handlers register in `service.go` `prepareHandlers` via `g.usc.inventory.ItemHandlers().Register("<Handler>", impl)`. Registered: `ItemSkills`/`ManaPotion` (potions), `SoulShots`/`SpiritShot`/`BlessedSpiritShot` (+`BeastSoulShot`/`BeastSpiritShot`/`FishShots` = parked no-ops, no pet/fishing systems), `EnchantScrolls`, `ExtractableItems`, `Recipes`.
- **Rewards → client**: handlers that add items (extractable) use `ItemUseContext.Emit` so new/changed items ride the used item's `InventoryUpdate` (built in `handlers/client/equipment.go` `sendEquipmentUpdatePackets`).
- **Template fields** (parsed by `registry/itemtemplates.go`, epic xv8): `Handler`, `ItemSkills []{ID,Level}` (from `item_skill` `id-lvl;...`), `ReuseDelay`, `SharedReuseGroup`, `ImmediateEffect`, `QuestItem` (type2==QUEST), `CapsuledItems` (extractable), `Soulshots`/`Spiritshots` counts.
- **UseItem pre-checks** (5i0, before the fork): quest-item → `CANNOT_USE_QUEST_ITEMS`; dead → `S1_CANNOT_BE_USED`; reuse-cooldown → remaining-time SystemMessage. usecase is transport-free (returns `Messages`/`ReuseSync` specs; handler translates to packets).
- **Reuse timers** (6vj): `registry.ItemReuseRegistry` (in-memory per-char `map[charID]map[objectID]stamp`, shared-group aware, injectable clock, cleared on `WorldRegistry.RemovePlayer`). Armed in `useNonEquipItem` after `consumed==true`. `ExUseSharedGroupItem` (0xFE:0x4A) sent **only for `shared_reuse_group > 0`** — matches L2J + lineage2ts (both gate `group<=0 → return`). charID == player objectID.
- **Charged shots** (sew): `registry.ChargedShotRegistry` (per-weapon-objectID in-memory flag). Grade-check via `gradeSPlus` (S/S80/S84→S+). Visual = `MagicSkillUse` (0x48).
- **Enchant** (f16 + 629): two-step HF window flow. `EnchantScrolls` handler arms scroll in `registry.EnchantStateRegistry` + sends legacy `ChooseInventoryItem`; client opens window itself and sends `RequestExTryToPutEnchantTargetItem` (0xD0:0x4c) → server `ValidateTarget` → `ExPutEnchantTargetItemResult` (0xFE:0x81); `RequestEnchantItem` (0x5f) does the enchant; `RequestExCancelEnchantItem` (0xD0:0x4e) closes. Chances are **retail-exact** from `enchantItemGroups.xml`+`enchantItemData.xml` (`registry/enchantgroups.go`, per-enchant-level tables, scrollGroupId binding).

### INTERIM boundaries (replaced by the skill engine — l2go-2w8)
- **Potions don't cast a real skill.** `diu` reads the linked `item_skill`'s effect+power from `registry/skilleffects.go` (lazy loader) and restores HP/MP/CP directly via the game loop (`CmdRestoreStats` → `handleRestoreStats`). It broadcasts a **stop-gap `MagicSkillUse`** (the item's skill id/level) for the cast animation only — no HoT/duration/land-rate/conditions.
- **Soulshot charge is not applied to damage** (charge set/consumed + visual only; damage integration parked in l2go-77a).

### Known item gaps (open bugs)
- **l2go-28l**: reuse-cooldown sweep not drawn on the item icon even for a grouped item (10152) that DOES send `ExUseSharedGroupItem`. Top hypothesis: the sweep is on the **shortcut bar** slot, not the inventory-window icon, and shortcut registration (`RequestShortCutReg`, l2go-znj) isn't implemented — so there's nowhere to show it. Verify by seeding `character_shortcuts` or doing znj first.
- **l2go-1in**: `InventoryUpdate` writes 3 fixed enchant-options; HF/our `ItemList` write a variable count (0 for normal items). Latent for multi-item InventoryUpdate.
- **l2go-znj**: items can't be placed on the quick bar (RequestShortCutReg + ExAutoSoulShot not implemented).

## Character Persistence

- **Sole writer**: the game loop mutates `player.Character` progress (EXP/SP/level/HP) **without a lock**. Never read those fields from another goroutine — snapshot instead.
- **Async saver** (`service.go run()`): a goroutine drains `saveCh chan models.Character` and calls `charRepo.Update`. The loop enqueues **value-copy** snapshots (`PlayerWorldState.SnapshotCharacter`) so DB latency never stalls the tick and the write can't race the loop.
- **Autosave**: every 5 min the loop snapshots all online players to `saveCh` (timer inside `tick`, like region cleanup).
- **Level-up**: persisted immediately (`experience.go`, in the level-up branch).
- **Save-on-shutdown**: after `eg.Wait()` (loop stopped → progress stable) the saver is flushed, then `saveOnlinePlayersOnShutdown` writes the freshest snapshot under the registry lock, **then** the DB closes. Order matters: flush old queued copies before the authoritative shutdown save so a stale copy can't overwrite it.
- **Position**: `movement.UpdatePosition` writes **only** the in-memory registry (no per-move DB write — it fires on every move start/stop and ~1-2s standing ValidatePosition). Position reaches the DB via the same unified persist (autosave/shutdown/logout), baked into each snapshot from `PlayerWorldState.Position`. On crash, position is ≤5 min stale (same tolerance as EXP/HP).

## Goroutine Ownership (read before touching world state)

- **Player visibility → game loop.** `PlayerWorldState.KnownPlayers` is written **only** by the loop goroutine (`gameloop/visibility.go`: `reconcilePlayerVisibility` on movement/enter-world, `despawnPlayerFromAll` on disconnect, cleared on teleport). Handlers must **not** touch it — route through commands (`CmdPlayerEnteredWorld`, `CmdPlayerDisconnected`). Spawn/despawn is bidirectional (a stationary player still sees a mover) and driven off the authoritative server position.
- **NPC visibility → the player's own connection goroutine.** `KnownNPCs` is touched only by that player's handler (`updateNPCVisibility`, `establishNpcVisibility`). Never from the loop.
- **Visibility distance** is one shared pair in `registry/visibility.go`: `VisibilityWatchRadius` (3400, spawn) / `VisibilityForgetRadius` (3900, despawn) — hysteresis, L2J HF. `broadcastRadius` = forget. Change in one place.

## Account Name Canonicalization

- Account names are **case-insensitive**, stored/compared as **lowercase everywhere** (matches L2J). Normalize at every ingress with `models.NormalizeAccountName` (lowercase + trim).
- Single client ingress: `handleAuthLogin` normalizes `packet.Account` → `session.AccountName`; everything downstream (created characters, `GetPlayerByAccount`, `ConnectionRegistry` keys, ownership checks) inherits the canonical case. Migration `007` lowercased existing rows.
- Do NOT compare account names case-sensitively or sprinkle `LOWER()` per query — rely on the canonical form. `GetCount` keeps `LOWER()=LOWER()` defensively (login path).

## Known Limitations

- Movement speed computed per-character (base×DEX from race/class); item/buff modifiers are a no-op hook (`applyMoveSpeedBonus`) pending item-stats/skill systems
- Item type classification approximate (no full item template DB yet)
- Multi-packet handler covers only a few sub-opcodes of 50+
- No collision detection