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
- SkillList (0x5F) HF per-skill layout is `D(passive) D(level) D(id) C(disabled) C(enchanted)` — passive FIRST, id AFTER level, and the last two flags are bytes (C), not D. (Earlier code wrote `D(id) D(level) D(passive) D(disabled) D(enchantable)` — wrong order + sizes; latent because the list was always empty.)

## Skill Engine (epic l2go-z36)

- **Templates**: `registry.SkillData` (`registry/skilldata.go`) lazily parses the skill datapack (`data/stats/skills/NNNNN-NNNNN.xml`) into one `models.Skill` per (id, level). L2J hash `id*1021+level`; `GetSkill` clamps a too-high level down to the skill's max. Parser expands `<table>`/`<set>`/`<effects>`-scope wrappers, resolving `#table` refs per level. Effects are **collected** (`SkillEffect{Name,Scope,Params,Funcs}`), **not executed** — casting/effects are later phases (lu8/c8t). Built in `service.go` (`g.skillData`), shared roots with the interim `skillEffects`.
- **SkillList read path (afx)**: skills are granted at creation (`learnStartingSkills` → `character_skills`) and loaded at world entry. `world.go` `buildSkillListPacket` → `CharacterUseCase.GetCharacterSkills` → `buildSkillInfos` maps each to `outclient.SkillInfo`, resolving the **passive** flag from `SkillData.GetSkill(...).IsPassive()` (handler holds a `SkillTemplateSource`, wired via `SetSkillData`) and **enchanted** from `level > 100`. DB error → empty list, never blocks entry.
- **Auto-get skill grants (3ih)**: `registry.SkillTreeData` (global, `GetSkillTreeRegistry()`) parses `data/skillTrees/classSkillTree.xml` at startup. `AutoGetSkills(classID, level)` returns the `autoGet="true"` skills of the class **and its parent chain** (`parentClassId`) with `getLevel <= level`, deduped to the max level per skill id (L2J `getAvailableAutoGetSkills` over the complete tree). `CharacterUseCase.grantAutoGetSkills` writes the missing/upgraded ones (shared by creation and entry). **Creation** grants the starting set (replaces the old hardcoded `template.StartingSkills`, which had wrong/legacy ids). **World entry** `ReconcileAutoGetSkills` idempotently grants anything missing for the current level — covers pre-existing characters and offline level-ups. **Interim scope**: only the `autoGet` subset (Lucky, Common Craft, Expertise, ...); `learnedByNpc` skills (Power Strike, masteries) stay NPC-learned with SP costs (hv9 P5). **Live online level-up grant is deferred (l2go-pt6)** — level-up runs on the game loop (no blocking DB); level-up passives currently land on the next relog via the entry reconcile.
- **Stat modifier pipeline + passives (9ep)**: `models/statmods.go` — `StatModifier{Stat,Op,Val}` + `ApplyStatModifiers(ComputedStats, mods)` layer over the flat `ComputeStats`. Formula per stat: `(base + Σadd − Σsub) × Πmul ÷ Πdiv` (order-independent within a stat; the standard emu approximation, exact for %/flat buffs). Stat names match the datapack (`pAtk`/`pDef`/`runSpd`/`accCombat`/`rEvas`/`critRate`/…); names absent from `ComputedStats` (resistances, regen, pvp) are ignored. Modifiers live on `Character.StatMods` (runtime-only, `json:"-"`), populated at world entry by `applyPassiveModifiers` (`collectPassiveModifiers` → `models.PassiveModifiers` pulls PASSIVE-scope stat funcs from each learned passive skill's template). `ApplyStatModifiers` is applied in the two central compute helpers (`usecase.ComputeCharacterStats` → combat, `computeCharInfoStats` → CharInfo) plus the UserInfo and level-up paths, so all stat consumers reflect passives. **`nil` mods → stats unchanged** (no behavior change until passives are actually granted). **Currently inert in-game**: the hardcoded starter sets grant zero passives; real auto-get from `classSkillTree.xml` is deferred (hv9). Writer discipline for `StatMods` mirrors Character progress: set on the connection goroutine before `CmdPlayerEnteredWorld`; once live, the game loop is sole writer and packets snapshot.

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
- **EXP/SP rewards** (ejz): reward comes from the **datapack** `<acquire expRate=".." sp=".."/>` (parsed in `registry/npctemplates.go` → `NpcTemplate.RewardExp`/`RewardSp`). Base EXP = `level² × expRate` (L2J `getExpReward`); SP = raw `sp`. Distributed proportionally by hate (≈damage), then the server rate. **Level penalty is asymmetric** (`data.LevelPenalty`, L2J `calculateExpAndSp`): only when the player is >5 levels **above** the mob → `(5/6)^(diff-5)`; at/below/within-5 = full reward, no 1% floor. No `npcLevel²` synthesis. Uses UserInfo (64-bit) instead of StatusUpdate (32-bit truncation) for EXP updates.
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
- **Charged shots** (sew): `registry.ChargedShotRegistry` (per-weapon-objectID in-memory flag; stores the weapon grade id alongside the charge for the hit visual). Grade-check via `gradeSPlus` (S/S80/S84→S+). Activation visual = `MagicSkillUse` (0x48). **Soulshot damage integration (77a):** the swing (`gameloop` `NextAttackEvent.Execute`) snapshots the RHand weapon's charge, doubles pAtk before defence/crit/variance (L2J `ssboost`), sets the Attack `USESS|grade` hit flag, and spends the charge **once, only on a landed hit** (a miss keeps it). Charge is cleared on weapon unequip. Spiritshot stays a parked hook (no magic damage until the skill engine).
- **Auto-soulshot (btb):** `registry.AutoShotRegistry` (in-memory per-char active shot itemIds, reset on relog like L2J `_activeSoulShots`). `RequestAutoSoulShot` (0xD0:0x0d) toggles it: enable validates the item is owned + not a fishing shot (6535-6540), echoes `ExAutoSoulShot` (0xFE:0x0c) + `USE_OF_S1_WILL_BE_AUTO`, and recharges immediately; disable echoes off + `AUTO_USE_OF_S1_CANCELLED`. Recharge is proactive (L2J does it at the end of `onHitTimer`): `gameloop` `HitEvent.Execute` enqueues the attacker's charID onto an async `rechargeCh` sink (service.go goroutine, keeps DB off the tick) when the weapon is uncharged; the sink runs `InventoryUseCase.RechargeAutoShots` (soulshots only — `template.Handler=="SoulShots"`), which runs the shot handler in **Auto mode** (`ItemUseContext.Auto`: consume + charge + visual, no chat spam) and returns consumed stacks → `SendInventoryUpdate` refreshes the bag count. Running out auto-disables the shot + echoes `ExAutoSoulShot(off)`.
- **Enchant** (f16 + 629): two-step HF window flow. `EnchantScrolls` handler arms scroll in `registry.EnchantStateRegistry` + sends legacy `ChooseInventoryItem`; client opens window itself and sends `RequestExTryToPutEnchantTargetItem` (0xD0:0x4c) → server `ValidateTarget` → `ExPutEnchantTargetItemResult` (0xFE:0x81); `RequestEnchantItem` (0x5f) does the enchant; `RequestExCancelEnchantItem` (0xD0:0x4e) closes. Chances are **retail-exact** from `enchantItemGroups.xml`+`enchantItemData.xml` (`registry/enchantgroups.go`, per-enchant-level tables, scrollGroupId binding).

### INTERIM boundaries (replaced by the skill engine — l2go-2w8)
- **Potions don't cast a real skill.** `diu` reads the linked `item_skill`'s effect+power from `registry/skilleffects.go` (lazy loader) and restores HP/MP/CP directly via the game loop (`CmdRestoreStats` → `handleRestoreStats`). It broadcasts a **stop-gap `MagicSkillUse`** (the item's skill id/level) for the cast animation only — no HoT/duration/land-rate/conditions.

### Quick bar / shortcuts (znj, DONE)
- `RequestShortCutReg` (0x3d) persists a shortcut + echoes `ShortCutRegister` (0x44); `RequestShortCutDel` (0x3f) deletes (no client ack, L2J parity); `ShortCutInit` (0x45) loads from `character_shortcuts` on world entry (`world.go` `BuildShortCutPacket`). usecase in `character.go` (`GetShortcuts`/`SaveShortcut`/`DeleteShortcut`). `BuildShortCutInit` ITEM trailer is `H,H` (24 bytes) — not `D,D`. Repo `ON CONFLICT` matches the full PK incl. `class_index`; migration 009 relaxed the `level` CHECK to `>=0` (client sends level 0 for item shortcuts).
- **l2go-28l resolved by znj**: grouped-item reuse sweep now draws on the shortcut slot. Non-grouped items (reuse but no shared group, e.g. 1060) draw **no** sweep — L2J parity (`UseItem.sendSharedGroupUpdate` gates on `group>0`); reuse still enforced server-side + remaining-time SystemMessage.

### Known item gaps (open)
- **l2go-1in**: `InventoryUpdate` writes 3 fixed enchant-options; HF/our `ItemList` write a variable count (0 for normal items). Latent for multi-item InventoryUpdate.

### Attack packet hit flags (HF, 77a)
`packets/outclient/attack.go` uses L2J HF `Hit.java` bits: USESS `0x10` (OR'd with weapon grade id), CRIT `0x20`, SHLD `0x40`, MISS `0x80`. (Earlier values `0x01/0x02/0x04` were wrong; only CRIT matched.)

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

- Movement speed computed per-character (base×DEX from race/class). Skill/passive `runSpd`/`walkSpd` modifiers now flow through `ComputedStats` (9ep stat pipeline); the legacy `applyMoveSpeedBonus` hook is a redundant identity. Item move-speed bonuses still pending item-stats.
- Item type classification approximate (no full item template DB yet)
- Multi-packet handler covers only a few sub-opcodes of 50+
- No collision detection