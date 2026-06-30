# Intention-машина — Фаза 2 (часть 2): интеракция через intention + отмена по движению + чистка

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Перевести взаимодействие с NPC на ту же intention-модель, что и атака (серверное движение к цели + heartbeat вместо поллинга застывшей позиции), и сбрасывать intention при команде движения игрока (клик на землю), чтобы персонаж не возвращался к цели. Чинит l2go-pfr и l2go-8qy, частично l2go-p80.

**Architecture:** `handleInteractRequest` ставит intention `INTERACT` и запускает серверное движение к NPC; `InteractApproachEvent` становится interact-heartbeat (перепроверяет дистанцию по серверной позиции, перезапускает серверное движение, при входе в радиус — открывает диалог один раз). Новая команда `CmdMoveToLocation` от обработчика `MoveBackwardToLocation` сбрасывает боевое/интерактивное намерение (останавливает атаку, чистит interact-подход), ставя `MOVE_TO`. Завершается чисткой мёртвого кода после фазы 2a.

**Tech Stack:** Go 1.22, пакеты `internal/gameserver/{gameloop,registry,usecase,models,handlers/client}`.

## Global Constraints

- Go 1.22.0; модуль `github.com/VerTox/l2go`.
- GameLoop — single goroutine; intention/движение/combat игрока меняет только горутина gameloop (через команды и тик).
- Переиспользовать: `startMoveToTarget`, `setIntention`/`clearIntention`, `interactApproachOffset` (interact reach = base 36 + коллизии), `stopAttacker`, тиковую интерполяцию фазы 1.
- Interact-движение — серверное (как attack): `startMoveToTarget` выставляет `IsMoving`/dest; heartbeat перепроверяет по серверной позиции, НЕ поллит клиентскую.
- Диалог открывается РОВНО один раз на прибытие (как и раньше — через `interactPending` дедуп).
- Порог открытия диалога — `interactRange` (150, L2J INTERACTION_DISTANCE); offset движения — `interactApproachOffset` (~85), заведомо внутри 150.
- Не ломать атаку (фаза 2a) и движение по земле (фаза 1).
- Каждый коммит компилируется и проходит `go test ./...`. Команды — по одной, без `&&`/пайпов.

## Известный контекст (в коде ветки feat/intention-machine)

- `intention.go`: `setIntention`, `clearIntention`, `startMoveToTarget(player, npc, reach)`, `onMovementArrived` (attack=no-op, interact=заглушка), `beginAttackSwing`.
- `gameloop.go`: `handleInteractRequest` (interactPending дедуп, `approachTarget` + `InteractApproachEvent`), `stopAttacker` (теперь зовёт `clearIntention`), `handleAttackRequest` (intention ATTACK + heartbeat).
- `events.go`: `InteractApproachEvent` — поллинг: при out-of-range throttled `approachTarget` + reschedule 300ms; при in-range — `MoveToPawn`+`NpcHtmlMessage`. `NextAttackEvent` — combat-heartbeat (поле `RetryCount` теперь мёртвое).
- `combat_range.go`: `interactApproachOffset(player, npc) int`, `approachTarget`, `shouldResendMoveToPawn` (используется только в `InteractApproachEvent`).
- `handlers/client/movement.go`: `handleMoveBackwardToLocation` → `StartMovement` (usecase), затем шлёт `CmdPlayerMoved`.
- `commands.go`: команды; есть `CmdCancelAttack{CharID}`.

## File Structure

- Modify: `internal/gameserver/gameloop/commands.go` — добавить `CmdMoveToLocation`.
- Modify: `internal/gameserver/gameloop/gameloop.go` — `handleInteractRequest` на intention; обработчик `CmdMoveToLocation` (сброс intention/атаки); регистрация в `processCommand`.
- Modify: `internal/gameserver/gameloop/events.go` — `InteractApproachEvent` на серверное движение + heartbeat; убрать мёртвое поле `RetryCount` из `NextAttackEvent`.
- Modify: `internal/gameserver/gameloop/combat_range.go` — убрать `shouldResendMoveToPawn`, если станет неиспользуемой.
- Modify: `internal/gameserver/handlers/client/movement.go` — `handleMoveBackwardToLocation` шлёт `CmdMoveToLocation`.
- Modify: `internal/gameserver/CLAUDE.md` — обновить устаревшее описание combat-подхода.

---

### Task 1: Интеракция через intention + серверное движение к цели

**Files:**
- Modify: `internal/gameserver/gameloop/gameloop.go` (`handleInteractRequest`)
- Modify: `internal/gameserver/gameloop/events.go` (`InteractApproachEvent.Execute`)

**Interfaces:**
- Consumes: `setIntention`, `startMoveToTarget`, `interactApproachOffset`, `clearIntention`, `interactRange`, `interactPending`.
- Produces: interact, ведомый серверным движением + heartbeat; диалог по входу в `interactRange`.

**Контекст:** `handleInteractRequest` сейчас шлёт `approachTarget` (клиентский MoveToPawn) + планирует `InteractApproachEvent`, который поллит застывшую клиентскую позицию — тот же корень, что был у атаки. Переводим на серверное движение (как attack): `startMoveToTarget` (тик интерполирует), а `InteractApproachEvent` — heartbeat по серверной позиции.

- [ ] **Step 1: Перевести `handleInteractRequest` на intention + серверное движение**

В `internal/gameserver/gameloop/gameloop.go`, в `handleInteractRequest`, заменить блок начального approach:

Заменить:

```go
	gl.interactPending[cmd.CharID] = cmd.TargetObjectID

	// Начальный MoveToPawn запускает движение клиента к NPC. Стоп-дистанция —
	// collision-aware offset (interact base + радиусы коллизий), заведомо меньше
	// триггера интеракции (150), чтобы персонаж любого размера зашёл внутрь радиуса.
	gl.approachTarget(cmd.AccountName, player, npc, gl.interactApproachOffset(player, npc))

	gl.events.Schedule(&InteractApproachEvent{
		At:             time.Now().Add(300 * time.Millisecond),
		CharID:         cmd.CharID,
		TargetObjectID: cmd.TargetObjectID,
		AccountName:    cmd.AccountName,
	})
```

на:

```go
	gl.interactPending[cmd.CharID] = cmd.TargetObjectID

	// INTERACT intention + server-side movement toward the NPC (the tick interpolates
	// position). InteractApproachEvent is a heartbeat that re-checks distance against
	// the SERVER position and opens the dialogue on arrival — no stale-client polling.
	gl.setIntention(cmd.CharID, IntentionInteract, cmd.TargetObjectID)
	gl.startMoveToTarget(player, npc, gl.interactApproachOffset(player, npc))

	gl.events.Schedule(&InteractApproachEvent{
		At:             time.Now().Add(300 * time.Millisecond),
		CharID:         cmd.CharID,
		TargetObjectID: cmd.TargetObjectID,
		AccountName:    cmd.AccountName,
	})
```

- [ ] **Step 2: Переделать `InteractApproachEvent.Execute` в heartbeat поверх серверного движения**

В `internal/gameserver/gameloop/events.go`, в `InteractApproachEvent.Execute`, заменить out-of-range блок:

Заменить:

```go
	dx := player.Position.X - npc.Position.X
	dy := player.Position.Y - npc.Position.Y
	if dx*dx+dy*dy > interactRange*interactRange {
		const maxRetries = 30
		if e.RetryCount >= maxRetries {
			clearPending()
			return
		}
		// Keep chasing: re-issue MoveToPawn at most ~once per second (same throttle as
		// attack approach) so a client that stopped short or whose position lagged still
		// closes in, without stuttering from a move restart every tick.
		if shouldResendMoveToPawn(e.RetryCount) {
			gl.approachTarget(e.AccountName, player, npc, gl.interactApproachOffset(player, npc))
		}
		gl.events.Schedule(&InteractApproachEvent{
			At:             time.Now().Add(300 * time.Millisecond),
			CharID:         e.CharID,
			TargetObjectID: e.TargetObjectID,
			AccountName:    e.AccountName,
			RetryCount:     e.RetryCount + 1,
		})
		return
	}
```

на:

```go
	dx := player.Position.X - npc.Position.X
	dy := player.Position.Y - npc.Position.Y
	if dx*dx+dy*dy > interactRange*interactRange {
		// Out of range: drive server-side movement (tick interpolates) and re-check on
		// the next heartbeat. Distance is checked against the server position, not a
		// stale client packet. Mirrors the attack heartbeat. interactPending is the
		// cancellation guard: a move/retarget command clears it and this stops.
		if !player.IsMoving {
			gl.startMoveToTarget(player, npc, gl.interactApproachOffset(player, npc))
		}
		gl.events.Schedule(&InteractApproachEvent{
			At:             time.Now().Add(400 * time.Millisecond),
			CharID:         e.CharID,
			TargetObjectID: e.TargetObjectID,
			AccountName:    e.AccountName,
		})
		return
	}
```

Примечание: поле `RetryCount` у `InteractApproachEvent` больше не используется. Удали его из определения структуры `InteractApproachEvent` (events.go) и из всех литералов (`InteractApproachEvent{...}` в gameloop.go и events.go) в рамках этой задачи — оно локально для interact-пути и его удаление не затрагивает attack.

- [ ] **Step 3: Сборка**

Run: `go build ./...`
Expected: BUILD OK. Если `shouldResendMoveToPawn` стала неиспользуемой — НЕ трогай в этой задаче (удаление в Task 3); компиляции Go это не мешает (метод, не локальная переменная).

- [ ] **Step 4: Полный прогон тестов**

Run: `go test ./...`
Expected: все пакеты ok / no test files.

- [ ] **Step 5: Коммит**

```bash
gofmt -w internal/gameserver/gameloop/gameloop.go internal/gameserver/gameloop/events.go
git add internal/gameserver/gameloop/gameloop.go internal/gameserver/gameloop/events.go
git commit -m "feat(gameloop): drive NPC interaction via intention + server move-to-target (fixes l2go-8qy)"
```

---

### Task 2: Отмена intention по команде движения (клик на землю)

**Files:**
- Modify: `internal/gameserver/gameloop/commands.go` (новая `CmdMoveToLocation`)
- Modify: `internal/gameserver/gameloop/gameloop.go` (`processCommand` + `handleMoveToLocation`)
- Modify: `internal/gameserver/handlers/client/movement.go` (`handleMoveBackwardToLocation` шлёт команду)

**Interfaces:**
- Produces:
  - `type CmdMoveToLocation struct { CharID int32 }` + `commandMarker()`.
  - `func (gl *GameLoop) handleMoveToLocation(cmd CmdMoveToLocation)` — сбрасывает боевое/интерактивное намерение: если игрок атаковал — `stopAttacker` (он уже зовёт `clearIntention`); иначе `clearIntention`; чистит `interactPending`; ставит `IntentionMoveTo`.

**Контекст:** клик на землю (`MoveBackwardToLocation`) во время атаки/интеракции сейчас не сбрасывает intention, и combat/interact-heartbeat тянет персонажа обратно к цели (l2go-pfr). В L2J команда движения меняет intention на MOVE_TO, прерывая ATTACK/INTERACT.

- [ ] **Step 1: Добавить команду `CmdMoveToLocation`**

В `internal/gameserver/gameloop/commands.go` добавить:

```go
// CmdMoveToLocation — player issued a ground move (clicked the ground). Cancels any
// attack/interact intention so the loop stops chasing the previous target.
type CmdMoveToLocation struct {
	CharID int32
}

func (CmdMoveToLocation) commandMarker() {}
```

- [ ] **Step 2: Обработать команду в gameloop**

В `internal/gameserver/gameloop/gameloop.go`, в `processCommand`, добавить case:

```go
	case CmdMoveToLocation:
		gl.handleMoveToLocation(c)
```

И добавить метод (рядом с `handleCancelAttack`):

```go
// handleMoveToLocation cancels attack/interact intention when the player issues a
// ground move, so the combat/interact heartbeat stops chasing the old target.
func (gl *GameLoop) handleMoveToLocation(cmd CmdMoveToLocation) {
	// Stop auto-attack if active (stopAttacker also clears intention).
	if cs, ok := gl.combatState[cmd.CharID]; ok && cs.IsAutoAttacking {
		gl.stopAttacker(cmd.CharID)
	} else {
		gl.clearIntention(cmd.CharID)
	}
	// Drop any pending interact approach.
	delete(gl.interactPending, cmd.CharID)
	gl.setIntention(cmd.CharID, IntentionMoveTo, 0)
}
```

- [ ] **Step 3: Слать команду из обработчика движения**

В `internal/gameserver/handlers/client/movement.go`, в `handleMoveBackwardToLocation`, после успешного старта движения (после отправки `MoveToLocation` подтверждения клиенту, рядом с существующей отправкой `CmdPlayerMoved` — найди где шлётся `h.gameLoopCmd`; если в этом обработчике его нет, добавь после `c.Send(moveConfirmation...)`):

```go
	// Ground move cancels attack/interact intention in the game loop (stop chasing).
	h.gameLoopCmd <- gameloop.CmdMoveToLocation{CharID: playerState.CharID}
```

ВАЖНО: проверь, что `gameloop` импортирован в movement.go (он уже используется для `CmdPlayerMoved` в `handleValidatePosition`) и что `playerState` доступен в этой точке.

- [ ] **Step 4: Сборка**

Run: `go build ./...`
Expected: BUILD OK.

- [ ] **Step 5: Полный прогон тестов**

Run: `go test ./...`
Expected: все пакеты ok / no test files.

- [ ] **Step 6: Коммит**

```bash
gofmt -w internal/gameserver/gameloop/commands.go internal/gameserver/gameloop/gameloop.go internal/gameserver/handlers/client/movement.go
git add internal/gameserver/gameloop/commands.go internal/gameserver/gameloop/gameloop.go internal/gameserver/handlers/client/movement.go
git commit -m "feat(gameloop): ground move cancels attack/interact intention (fixes l2go-pfr)"
```

---

### Task 3: Чистка мёртвого кода после фазы 2a/2b

**Files:**
- Modify: `internal/gameserver/gameloop/events.go` (убрать `RetryCount` из `NextAttackEvent`)
- Modify: `internal/gameserver/gameloop/combat_range.go` (убрать `shouldResendMoveToPawn`, если не используется)
- Modify: `internal/gameserver/CLAUDE.md` (обновить устаревшее описание)

**Interfaces:** только удаления и правка доков; новых символов нет.

- [ ] **Step 1: Убрать мёртвое поле `RetryCount` из `NextAttackEvent`**

Проверить, что поле больше не читается/пишется: `grep -n "RetryCount" internal/gameserver/gameloop/events.go internal/gameserver/gameloop/gameloop.go`
В `events.go`, в определении `type NextAttackEvent struct`, удалить строку:

```go
	RetryCount     int // number of distance-check retries (max 30 ≈ 9s)
```

И убедиться, что ни один литерал `NextAttackEvent{...}` не задаёт `RetryCount` (после фазы 2a их быть не должно). Если есть — удалить это поле из литерала.

- [ ] **Step 2: Убрать `shouldResendMoveToPawn`, если не используется**

Проверить: `grep -rn "shouldResendMoveToPawn" internal/gameserver/`
Если используется ТОЛЬКО в собственном определении (после Task 1 interact на неё не ссылается) — удалить функцию `shouldResendMoveToPawn` из `combat_range.go`. Если ещё где-то используется — оставить и отметить в отчёте.

- [ ] **Step 3: Сборка**

Run: `go build ./...`
Expected: BUILD OK.

- [ ] **Step 4: Обновить `internal/gameserver/CLAUDE.md`**

Найти устаревшие описания combat-подхода и заменить на актуальные. Заменить строку:

```
- **Auto-attack**: `CmdAttackRequest` → `NextAttackEvent` (with 500ms initial delay for approach) → `HitEvent` (damage at mid-swing) → next swing cycle
```

на:

```
- **Auto-attack**: `CmdAttackRequest` sets ATTACK intention → server-side move-to-target (tick interpolates position) → `NextAttackEvent` combat-heartbeat checks reach against the server position → `HitEvent` (damage at mid-swing) → next swing cycle
```

И заменить строку:

```
- **Approach retry**: If player is out of melee range, `NextAttackEvent` retries every 300ms (max 30 attempts ≈ 9s) instead of immediately cancelling
```

на:

```
- **Approach**: out-of-reach `NextAttackEvent`/`InteractApproachEvent` restart server-side movement (`startMoveToTarget`) and re-check on the next heartbeat (~400ms); the server position (not stale client packets) drives arrival. A ground move cancels the intention (stops chasing).
```

- [ ] **Step 5: Полный прогон тестов**

Run: `go test ./...`
Expected: все пакеты ok / no test files.

- [ ] **Step 6: Коммит**

```bash
gofmt -w internal/gameserver/gameloop/events.go internal/gameserver/gameloop/combat_range.go
git add internal/gameserver/gameloop/events.go internal/gameserver/gameloop/combat_range.go internal/gameserver/CLAUDE.md
git commit -m "chore(gameloop): drop dead RetryCount/shouldResendMoveToPawn, refresh combat docs"
```

---

## Проверка в клиенте (после Task 2 и после Task 3)

Пересобрать GameServer (ветка `feat/intention-machine`):
1. **Диалог NPC** — двойной клик по NPC на разной дистанции (близко/далеко, крупные NPC): персонаж добегает и **диалог открывается стабильно** (не «через раз»).
2. **Отмена кликом на землю** — выбери моба/NPC, нажми атаку/интеракцию, и во время подхода кликни на землю в сторону: персонаж идёт к точке и **НЕ возвращается** к цели (l2go-pfr).
3. **Регрессия атаки** — атака по-прежнему работает (фаза 2a не сломана).

Примечание: баг `l2go-p80` (таргет висит после смерти моба + блок движения) этим планом адресуется лишь частично (сброс intention командой движения). Полная диагностика l2go-p80 — отдельно.

## Self-Review

- **Покрытие:** interact через intention + серверное движение (Task 1); отмена intention командой движения (Task 2, l2go-pfr); чистка мёртвого кода + доки (Task 3).
- **Плейсхолдеры:** нет. Места проверки фактического кода (наличие `gameLoopCmd`/`playerState` в movement.go; использование `shouldResendMoveToPawn`/`RetryCount`) помечены явными `grep`-командами.
- **Согласованность:** `CmdMoveToLocation`, `handleMoveToLocation`, `IntentionMoveTo`, `startMoveToTarget`, `interactApproachOffset`, `interactRange`, `interactPending` — единообразны; interact-heartbeat зеркалит attack-heartbeat (один чейн на charID, дедуп через `interactPending`).
- **Известные долги (ledger):** гонка по полям движения (тик vs handler) — не закрывается здесь; l2go-7qv (боевая стойка при нажатии) — отдельный баг; l2go-p80 — отдельная диагностика.
