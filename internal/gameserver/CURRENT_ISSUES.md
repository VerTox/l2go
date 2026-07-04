# Current Issues and TODOs — L2Go GameServer

**Last Updated**: 2026-01-28

## Resolved Issues

### Attack not reaching target (fixed 2026-01-28)
- `MoveBackwardToLocation` was sending `CmdCancelAttack`, cancelling attack when client moves to approach NPC
- `NextAttackEvent` immediately stopped attack when out of range instead of retrying
- First attack was scheduled at `time.Now()` before player could approach
- **Fix**: Removed CmdCancelAttack from movement, added 300ms retry (max 30), 500ms initial delay

### EXP bar not updating (fixed 2026-01-28)
- `StatusUpdate` uses `WriteD` (32-bit) for EXP value, truncating `int64` Experience at higher levels
- **Fix**: Replaced StatusUpdate with UserInfo packet (uses `WriteQ` 64-bit) for EXP/Level/SP. StatusUpdate kept only for HP/MP

### Combat stance not visible + no logout block (fixed 2026-01-28)
- `CharInfo.InCombat` and `UserInfo.InCombat` were hardcoded to 0
- Logout proceeded even during combat despite validation failing
- **Fix**: InCombat read from `PlayerWorldState`, UserInfo sent on combat state changes, logout blocked with SystemMessage(1116)

## Open Issues

### Movement Speed System
**Status**: Hardcoded, needs character-based calculation
- Fixed speeds (80 walk, 120 run) in movement_validation.go
- Should calculate from: race/class templates, equipment, buffs, level/stats
- Files: `usecase/movement_validation.go`

### Item Type Classification
**Status**: Approximate
- Item types detected by ID ranges, not from actual item database
- Needs proper item template system for accurate classification
- Files: `handlers/client/world.go`

### Multi-packet Handler Gaps
**Status**: Basic implementation
- Only a few sub-opcodes of 50+ implemented (0x0d, 0x21, 0x22)
- Files: `handlers/client/multipacket.go`

## Technical Debt

- Some large files could be split further
- Mixed configuration styles (.env vs JSON)
- Some hardcoded values should be configurable
- No collision detection or obstacle checking
- No integration/load tests for packet flows

## Status Summary

**Working**: Auth, character management, world entry, equipment display, player visibility, movement, NPCs (~38K), targeting (with HP bar), NPC basic dialogue, combat (auto-attack, EXP/SP, death/respawn, combat stance), logout/restart (with combat block)

**Not implemented**: NPC trading/teleportation, skills, full action list (1/189), quests, parties
