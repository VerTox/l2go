# Intention-машина — Фаза 2 (часть 1): атака через intention + серверное движение к цели

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Перевести auto-attack на intention-модель: gameloop сам ведёт серверное движение игрока к цели (выставляет `IsMoving`/`MoveDestination`, тик интерполирует через механизм фазы 1), а удар начинается по **серверному** прибытию (`onArrived`), а не по поллингу застывшей клиентской позиции. Это чинит «атака начинается только после клика на землю» (l2go-2za).

**Architecture:** Новый `PlayerAIState` (intention + цель) в gameloop, по charID. При запросе атаки ставится intention `ATTACK`; если цель вне reach — стартует серверное движение к точке на границе reach (как `StartMovement`: `IsMoving=true`, `MoveStartPos/MoveDestination/MoveStarted`). Тик (`advancePlayerMovement`, фаза 1) двигает игрока и при прибытии вызывает `onMovementArrived`, который по intention начинает swing-цикл. `NextAttackEvent` перестаёт поллить approach: если в reach — бьёт, если цель ушла из reach — перезапускает серверное движение.

**Tech Stack:** Go 1.22, пакеты `internal/gameserver/{gameloop,registry,usecase,models}`, тесты `go test`.

## Global Constraints

- Go 1.22.0; модуль `github.com/VerTox/l2go`.
- GameLoop — single goroutine; intention/движение/combat игрока читает и пишет только горутина gameloop (через команды и тик).
- Переиспользовать механизм фазы 1: `advancePlayerMovement`/`stepPlayerMovement`/`interpolatePosition`, `usecase.CalculateMovementTime`.
- Переиспользовать `attackReach`/`meleeReach` (collision-aware reach) из combat_range.go.
- Серверное движение к цели согласовано с клиентским: gameloop шлёт `MoveToPawn` (через `approachTarget`) И стартует своё движение; `onArrived` — по серверной интерполяции (сервер-авторитет).
- Не ломать механизм фазы 1 (движение по клику на землю), broadcast, drift-коррекцию в покое.
- Каждый коммит компилируется и проходит `go test ./...`. Команды запускать по одной, без склейки `&&`/пайпов.

## Известный контекст (из фазы 1, в коде ветки feat/intention-machine)

- `advancePlayerMovement(now)` в `tick()` двигает игроков с `IsMoving`, при прибытии чистит `IsMoving`/`MoveStartPos`/`MoveDestination` (gameloop.go).
- `stepPlayerMovement(player, now) (models.Position, bool)`, `interpolatePosition`, `distanceBetween` (movement_interp.go).
- `meleeReach(player, npc) int`, `approachTarget(accountName, player, npc, reach)` (combat_range.go).
- `PlayerCombatState{IsAutoAttacking, TargetObjectID, LastAttackTime, AccountName}` в `gl.combatState` (gameloop.go).
- `NextAttackEvent` (events.go): сейчас проверяет reach, при out-of-range делает throttled approach + retry; при in-range — swing (damage/HitEvent/next swing).
- `handleAttackRequest` (gameloop.go): создаёт combatState, шлёт UserInfo/AutoAttackStart, начальный approachTarget, schedule первого NextAttackEvent (+500мс).
- `registry.PlayerWorldState`: поля `IsMoving, MoveStarted, MoveStartPos, MoveDestination, IsRunning, Position, Heading, CharID, AccountName`.

## Принятые дизайн-решения

- **Отдельный `PlayerAIState`** (не расширяем `combatState`): intention — более общая концепция, чем swing-состояние; в части 2 он же обслужит interact/cast/follow.
- **Точка остановки** при движении к цели: на линии цель→игрок, на расстоянии reach от цели (чуть внутри, чтобы по прибытии гарантированно `dist <= reach`). Чистая функция `stopPointWithinReach`.
- **onArrived — по серверу.** При рассинхроне серверной (хардкод 120/80) и клиентской скорости сервер может «прибыть» раньше/позже клиента — действие всё равно сработает (сервер-авторитет), что и есть цель: больше нет зависимости от застывшей клиентской позиции. Точную скорость даёт отдельная задача l2go-tl9.1.

## File Structure

- Create: `internal/gameserver/gameloop/intention.go` — тип `Intention`, `PlayerAIState`, `setIntention`, `clearIntention`, `stopPointWithinReach`, `startMoveToTarget`, `onMovementArrived`.
- Create: `internal/gameserver/gameloop/intention_test.go` — тесты чистых функций и переходов.
- Modify: `internal/gameserver/gameloop/gameloop.go` — поле `aiState map[int32]*PlayerAIState` в struct и `New`; `advancePlayerMovement` вызывает `onMovementArrived` при прибытии; `handleAttackRequest` переведён на intention.
- Modify: `internal/gameserver/gameloop/events.go` — `NextAttackEvent` без approach-поллинга (re-move через `startMoveToTarget`).

---

### Task 1: Intention-состояние и геометрия точки остановки

**Files:**
- Create: `internal/gameserver/gameloop/intention.go`
- Create: `internal/gameserver/gameloop/intention_test.go`
- Modify: `internal/gameserver/gameloop/gameloop.go` (поле `aiState` в struct и инициализация в `New`)

**Interfaces:**
- Produces:
  - `type Intention int` с константами `IntentionIdle, IntentionMoveTo, IntentionAttack, IntentionInteract, IntentionCast, IntentionFollow`.
  - `type PlayerAIState struct { Intention Intention; TargetObjectID int32; MoveDest models.Position }`
  - `func (gl *GameLoop) setIntention(charID int32, intention Intention, targetObjectID int32)`
  - `func (gl *GameLoop) clearIntention(charID int32)`
  - `func stopPointWithinReach(from, target models.Position, reach int) models.Position` — точка на линии target→from на расстоянии reach от target (если from ближе reach — возвращает from).
  - Поле `gl.aiState map[int32]*PlayerAIState`.

- [ ] **Step 1: Написать падающий тест `stopPointWithinReach`**

Создать `internal/gameserver/gameloop/intention_test.go`:

```go
package gameloop

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

func TestStopPointWithinReach(t *testing.T) {
	target := models.Position{X: 0, Y: 0, Z: 100}

	t.Run("from far on +X axis → point at reach distance on +X", func(t *testing.T) {
		from := models.Position{X: 1000, Y: 0, Z: 100}
		p := stopPointWithinReach(from, target, 80)
		if p.X != 80 || p.Y != 0 {
			t.Errorf("got %+v, want X=80 Y=0", p)
		}
	})

	t.Run("already within reach → returns from unchanged", func(t *testing.T) {
		from := models.Position{X: 50, Y: 0, Z: 100}
		p := stopPointWithinReach(from, target, 80)
		if p != from {
			t.Errorf("got %+v, want unchanged %+v", p, from)
		}
	})

	t.Run("diagonal → preserves direction, distance≈reach", func(t *testing.T) {
		from := models.Position{X: 300, Y: 400, Z: 100} // dist 500
		p := stopPointWithinReach(from, target, 100)
		// direction (0.6, 0.8) * 100 = (60, 80)
		if p.X != 60 || p.Y != 80 {
			t.Errorf("got %+v, want X=60 Y=80", p)
		}
	})
}
```

- [ ] **Step 2: Запустить — убедиться, что падает**

Run: `go test ./internal/gameserver/gameloop/ -run TestStopPointWithinReach -v`
Expected: FAIL — `undefined: stopPointWithinReach`.

- [ ] **Step 3: Реализовать `intention.go`**

Создать `internal/gameserver/gameloop/intention.go`:

```go
package gameloop

import (
	"math"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// Intention is the player's current AI intention (L2J AI_INTENTION_*).
type Intention int

const (
	IntentionIdle Intention = iota
	IntentionMoveTo
	IntentionAttack
	IntentionInteract
	IntentionCast   // scaffold — skill system not implemented yet
	IntentionFollow // scaffold — follow not implemented yet
)

// PlayerAIState holds a player's current intention and its target.
type PlayerAIState struct {
	Intention      Intention
	TargetObjectID int32           // for Attack / Interact / Cast
	MoveDest       models.Position // for MoveTo
}

// setIntention records the player's intention and target, replacing any previous one.
func (gl *GameLoop) setIntention(charID int32, intention Intention, targetObjectID int32) {
	st, ok := gl.aiState[charID]
	if !ok {
		st = &PlayerAIState{}
		gl.aiState[charID] = st
	}
	st.Intention = intention
	st.TargetObjectID = targetObjectID
}

// clearIntention resets the player to idle.
func (gl *GameLoop) clearIntention(charID int32) {
	if st, ok := gl.aiState[charID]; ok {
		st.Intention = IntentionIdle
		st.TargetObjectID = 0
	}
}

// stopPointWithinReach returns the point on the line from target toward `from`
// at distance `reach` from target — i.e. where the mover should stop to be just
// within reach of target. If `from` is already within reach, returns `from`.
func stopPointWithinReach(from, target models.Position, reach int) models.Position {
	dx := float64(from.X - target.X)
	dy := float64(from.Y - target.Y)
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist <= float64(reach) || dist == 0 {
		return from
	}
	ratio := float64(reach) / dist
	return models.Position{
		X: target.X + int(dx*ratio),
		Y: target.Y + int(dy*ratio),
		Z: from.Z,
	}
}
```

- [ ] **Step 4: Запустить — убедиться, что проходит**

Run: `go test ./internal/gameserver/gameloop/ -run TestStopPointWithinReach -v`
Expected: PASS.

- [ ] **Step 5: Добавить `aiState` в GameLoop struct и `New`**

В `internal/gameserver/gameloop/gameloop.go`, в `type GameLoop struct`, рядом с `combatState`:

```go
	combatState     map[int32]*PlayerCombatState // charID -> auto-attack state
	aiState         map[int32]*PlayerAIState     // charID -> current intention
```

В функции `New`, рядом с `combatState: make(...)`:

```go
		combatState:     make(map[int32]*PlayerCombatState),
		aiState:         make(map[int32]*PlayerAIState),
```

- [ ] **Step 6: Написать падающий тест переходов intention**

Добавить в `intention_test.go`:

```go
func TestSetAndClearIntention(t *testing.T) {
	gl := &GameLoop{aiState: make(map[int32]*PlayerAIState)}

	gl.setIntention(7, IntentionAttack, 1003)
	st := gl.aiState[7]
	if st == nil || st.Intention != IntentionAttack || st.TargetObjectID != 1003 {
		t.Fatalf("setIntention not stored: %+v", st)
	}

	// overwrite replaces previous intention/target
	gl.setIntention(7, IntentionInteract, 2004)
	if st.Intention != IntentionInteract || st.TargetObjectID != 2004 {
		t.Errorf("setIntention did not overwrite: %+v", st)
	}

	gl.clearIntention(7)
	if st.Intention != IntentionIdle || st.TargetObjectID != 0 {
		t.Errorf("clearIntention did not reset: %+v", st)
	}
}
```

- [ ] **Step 7: Запустить — убедиться, что проходит**

Run: `go test ./internal/gameserver/gameloop/ -run 'TestSetAndClearIntention|TestStopPointWithinReach' -v`
Expected: PASS (тест переходов проходит сразу — функции уже реализованы в Step 3; это проверка интеграции с реальным GameLoop struct, а не TDD-red). Если struct/поле не компилируется — исправить Step 5.

- [ ] **Step 8: Сборка пакета**

Run: `go build ./...`
Expected: BUILD OK.

- [ ] **Step 9: Коммит**

```bash
gofmt -w internal/gameserver/gameloop/intention.go internal/gameserver/gameloop/intention_test.go internal/gameserver/gameloop/gameloop.go
git add internal/gameserver/gameloop/intention.go internal/gameserver/gameloop/intention_test.go internal/gameserver/gameloop/gameloop.go
git commit -m "feat(gameloop): player intention state and reach stop-point geometry"
```

---

### Task 2: Серверное движение к цели + onArrived-диспетчер

**Files:**
- Modify: `internal/gameserver/gameloop/intention.go` (`startMoveToTarget`, `onMovementArrived`)
- Modify: `internal/gameserver/gameloop/gameloop.go` (`advancePlayerMovement` вызывает `onMovementArrived` при прибытии)
- Test: `internal/gameserver/gameloop/intention_test.go`

**Interfaces:**
- Consumes: `stopPointWithinReach`, `meleeReach`, `approachTarget` (combat_range.go), `gl.aiState`, `registry.PlayerWorldState`, `models.NpcInstance`.
- Produces:
  - `func (gl *GameLoop) startMoveToTarget(player *registry.PlayerWorldState, npc *models.NpcInstance, reach int)` — выставляет `IsMoving=true`, `MoveStartPos=player.Position`, `MoveDestination=stopPointWithinReach(...)`, `MoveStarted=now`; шлёт `approachTarget` (MoveToPawn). Если игрок уже в reach — ничего не делает.
  - `func (gl *GameLoop) onMovementArrived(charID int32)` — вызывается тиком при завершении движения; диспетчеризует по intention (в этой части — только `IntentionAttack` → старт swing-цикла через `beginAttackSwing`; заглушки для остальных).
  - `func (gl *GameLoop) beginAttackSwing(charID int32, targetObjectID int32)` — планирует немедленный `NextAttackEvent` (без задержки approach).

**Контекст:** в `advancePlayerMovement` (gameloop.go, фаза 1) ветка `if arrived { ... }` сейчас только чистит поля движения. Нужно после очистки вызвать `gl.onMovementArrived(charID)`.

- [ ] **Step 1: Написать падающий тест `startMoveToTarget`**

Добавить в `intention_test.go` (импорт `registry`, `models` уже есть; добавить `github.com/VerTox/l2go/internal/gameserver/registry`):

```go
func TestStartMoveToTargetSetsServerMovement(t *testing.T) {
	gl := &GameLoop{
		aiState:     make(map[int32]*PlayerAIState),
		connections: registry.NewConnectionRegistry(), // no conn for charID → approachTarget is a no-op send
	}
	player := &registry.PlayerWorldState{
		CharID:   7,
		Position: models.Position{X: 1000, Y: 0, Z: 100},
		IsRunning: true,
	}
	npc := &models.NpcInstance{ObjectID: 1003, Position: models.Position{X: 0, Y: 0, Z: 100}}

	gl.startMoveToTarget(player, npc, 80)

	if !player.IsMoving {
		t.Fatal("expected IsMoving=true after startMoveToTarget")
	}
	if player.MoveStartPos != (models.Position{X: 1000, Y: 0, Z: 100}) {
		t.Errorf("MoveStartPos = %+v, want player start", player.MoveStartPos)
	}
	// destination is on +X axis at reach 80 from target
	if player.MoveDestination.X != 80 || player.MoveDestination.Y != 0 {
		t.Errorf("MoveDestination = %+v, want X=80 Y=0", player.MoveDestination)
	}
}

func TestStartMoveToTargetNoopWhenInReach(t *testing.T) {
	gl := &GameLoop{
		aiState:     make(map[int32]*PlayerAIState),
		connections: registry.NewConnectionRegistry(),
	}
	player := &registry.PlayerWorldState{
		CharID:   7,
		Position: models.Position{X: 50, Y: 0, Z: 100},
	}
	npc := &models.NpcInstance{ObjectID: 1003, Position: models.Position{X: 0, Y: 0, Z: 100}}

	gl.startMoveToTarget(player, npc, 80)

	if player.IsMoving {
		t.Error("expected no movement when already within reach")
	}
}
```

ВАЖНО: проверь точное имя конструктора реестра соединений (`registry.NewConnectionRegistry` или аналог) командой `grep -n "func New.*ConnectionRegistry" internal/gameserver/registry/connections.go` и используй фактическое. Если конструктор требует аргументы — создай минимально валидный экземпляр.

- [ ] **Step 2: Запустить — убедиться, что падает**

Run: `go test ./internal/gameserver/gameloop/ -run TestStartMoveToTarget -v`
Expected: FAIL — `undefined: startMoveToTarget` (или ошибка компиляции теста, если имя конструктора реестра иное — сперва поправь конструктор в тесте).

- [ ] **Step 3: Реализовать `startMoveToTarget`, `onMovementArrived`, `beginAttackSwing`**

Добавить в `internal/gameserver/gameloop/intention.go` (добавить импорты `time`, `registry`):

```go
// startMoveToTarget begins SERVER-SIDE movement of the player toward npc, stopping
// within `reach`. It sets the movement fields the tick interpolation (phase 1) reads,
// and sends a MoveToPawn so the client walks the same path. No-op if already in reach.
func (gl *GameLoop) startMoveToTarget(player *registry.PlayerWorldState, npc *models.NpcInstance, reach int) {
	dest := stopPointWithinReach(player.Position, npc.Position, reach)
	if dest == player.Position {
		return // already within reach
	}
	player.IsMoving = true
	player.MoveStartPos = player.Position
	player.MoveDestination = dest
	player.MoveStarted = gl.now()
	gl.approachTarget(player.AccountName, player, npc, reach)
}

// onMovementArrived is called by the tick when a player's server-side movement
// completes. It dispatches on the player's current intention.
func (gl *GameLoop) onMovementArrived(charID int32) {
	st, ok := gl.aiState[charID]
	if !ok {
		return
	}
	switch st.Intention {
	case IntentionAttack:
		gl.beginAttackSwing(charID, st.TargetObjectID)
	case IntentionInteract:
		// handled in phase 2 part 2
	default:
		// MoveTo / Idle / scaffolded intentions: nothing to do on arrival
	}
}

// beginAttackSwing schedules an immediate attack swing (no approach delay); range
// is re-checked inside NextAttackEvent.
func (gl *GameLoop) beginAttackSwing(charID int32, targetObjectID int32) {
	cs, ok := gl.combatState[charID]
	if !ok || !cs.IsAutoAttacking || cs.TargetObjectID != targetObjectID {
		return
	}
	gl.events.Schedule(&NextAttackEvent{
		At:             gl.now(),
		AttackerCharID: charID,
		TargetObjectID: targetObjectID,
	})
}
```

Добавить хелпер времени в `intention.go` (или gameloop.go, если ещё нет) — для тестируемости и единообразия:

```go
// now returns the current time. Indirection kept minimal for future testability.
func (gl *GameLoop) now() time.Time { return time.Now() }
```

ЕСЛИ в gameloop уже есть способ получать время — используй его и не добавляй `now()`; тогда в коде выше замени `gl.now()` на `time.Now()`. Проверь: `grep -n "time.Now()" internal/gameserver/gameloop/gameloop.go | head`.

- [ ] **Step 4: Вызвать `onMovementArrived` в `advancePlayerMovement`**

В `internal/gameserver/gameloop/gameloop.go`, в `advancePlayerMovement`, ветка `if arrived`:

```go
		if arrived {
			player.IsMoving = false
			player.MoveStartPos = models.Position{}
			player.MoveDestination = models.Position{}
			gl.onMovementArrived(charID)
		}
```

- [ ] **Step 5: Запустить тесты и сборку**

Run: `go test ./internal/gameserver/gameloop/ -run TestStartMoveToTarget -v`
Expected: PASS.
Затем отдельной командой: `go build ./...`
Expected: BUILD OK.

- [ ] **Step 6: Коммит**

```bash
gofmt -w internal/gameserver/gameloop/intention.go internal/gameserver/gameloop/gameloop.go
git add internal/gameserver/gameloop/intention.go internal/gameserver/gameloop/intention_test.go internal/gameserver/gameloop/gameloop.go
git commit -m "feat(gameloop): server-side move-to-target and onArrived intention dispatch"
```

---

### Task 3: Перевод auto-attack на intention (e2e)

**Files:**
- Modify: `internal/gameserver/gameloop/gameloop.go` (`handleAttackRequest`)
- Modify: `internal/gameserver/gameloop/events.go` (`NextAttackEvent.Execute` — убрать approach-поллинг, re-move через `startMoveToTarget`)

**Interfaces:**
- Consumes: `setIntention`, `startMoveToTarget`, `meleeReach`, `beginAttackSwing` (Task 1-2); существующие `combatState`, swing-логика `NextAttackEvent`.
- Produces: атака, начинающаяся по серверному прибытию; `NextAttackEvent` без поллинга approach.

**Контекст:** сейчас `handleAttackRequest` создаёт combatState, шлёт начальный `approachTarget` и планирует `NextAttackEvent(+500мс)`. `NextAttackEvent` при out-of-range делает throttled `approachTarget` + retry каждые 300мс (поллинг застывшей позиции — корень бага). Переводим на intention + серверное движение.

- [ ] **Step 1: Перевести `handleAttackRequest` на intention + серверное движение**

В `internal/gameserver/gameloop/gameloop.go`, в `handleAttackRequest`, заменить блок начального approach и планирования первого удара:

Заменить:

```go
	// Send the initial MoveToPawn so the client immediately walks into attack reach.
	// Done here (not in the packet handler) so the move offset uses the same reach
	// formula as the hit-range check, keeping client and server in sync (L2J: the AI
	// owns movement). NextAttackEvent re-issues it while out of range.
	if player, exists := gl.world.GetPlayer(cmd.AttackerCharID); exists {
		reach := gl.meleeReach(player, npc)
		dx := player.Position.X - npc.Position.X
		dy := player.Position.Y - npc.Position.Y
		if dx*dx+dy*dy > reach*reach {
			gl.approachTarget(cmd.AccountName, player, npc, reach)
		}
	}

	// Schedule first attack with a small delay to let the client approach the target
	gl.events.Schedule(&NextAttackEvent{
		At:             time.Now().Add(500 * time.Millisecond),
		AttackerCharID: cmd.AttackerCharID,
		TargetObjectID: cmd.TargetObjectID,
	})
```

на:

```go
	// Set ATTACK intention. If the target is out of reach, start SERVER-SIDE movement
	// toward it: the tick interpolates the position and fires onMovementArrived (which
	// begins the swing) on server arrival — no dependency on stale client position.
	// If already in reach, begin swinging immediately.
	gl.setIntention(cmd.AttackerCharID, IntentionAttack, cmd.TargetObjectID)
	if player, exists := gl.world.GetPlayer(cmd.AttackerCharID); exists {
		reach := gl.meleeReach(player, npc)
		dx := player.Position.X - npc.Position.X
		dy := player.Position.Y - npc.Position.Y
		if dx*dx+dy*dy > reach*reach {
			gl.startMoveToTarget(player, npc, reach)
		} else {
			gl.beginAttackSwing(cmd.AttackerCharID, cmd.TargetObjectID)
		}
	}
```

- [ ] **Step 2: Убрать approach-поллинг из `NextAttackEvent`, заменить на re-move**

В `internal/gameserver/gameloop/events.go`, в `NextAttackEvent.Execute`, блок out-of-range:

Заменить:

```go
	if distSq > rangeSq {
		// Out of range — keep chasing (re-issue MoveToPawn) and retry (~9 seconds).
		const maxRetries = 30
		if e.RetryCount >= maxRetries {
			gl.stopAttacker(e.AttackerCharID)
			return
		}
		// Re-issue MoveToPawn at most ~once per second (L2J _moveToPawnTimeout) so the
		// client keeps closing in without stuttering from a move restart every tick.
		if shouldResendMoveToPawn(e.RetryCount) {
			if cs, ok := gl.combatState[e.AttackerCharID]; ok {
				gl.approachTarget(cs.AccountName, player, npc, reach)
			}
		}
		gl.events.Schedule(&NextAttackEvent{
			At:             time.Now().Add(300 * time.Millisecond),
			AttackerCharID: e.AttackerCharID,
			TargetObjectID: e.TargetObjectID,
			RetryCount:     e.RetryCount + 1,
		})
		return
	}
```

на:

```go
	if distSq > rangeSq {
		// Target out of reach (it moved, or we never arrived). Restart server-side
		// movement toward it; onMovementArrived will resume the swing. No polling of
		// stale client position — the tick drives both position and arrival.
		if !player.IsMoving {
			gl.startMoveToTarget(player, npc, reach)
		}
		return
	}
```

Примечание: `shouldResendMoveToPawn` может стать неиспользуемой после части 2 (interact ещё её использует). НЕ удаляй её в этой задаче, если она используется в InteractApproachEvent — проверь `grep -rn "shouldResendMoveToPawn" internal/gameserver/gameloop/`. Если используется только здесь — оставь, удаление вне scope этой задачи (зафиксируй в отчёте как кандидат на чистку в части 2).

- [ ] **Step 3: Сборка**

Run: `go build ./...`
Expected: BUILD OK. Если `reach` или `rangeSq` стали неиспользуемыми из-за правки — проверь, что `reach` всё ещё вычисляется выше (`reach := gl.meleeReach(player, npc)`) и используется в `startMoveToTarget` и в `rangeSq`.

- [ ] **Step 4: Полный прогон тестов**

Run: `go test ./...`
Expected: все пакеты `ok` / `no test files`.

- [ ] **Step 5: Коммит**

```bash
gofmt -w internal/gameserver/gameloop/gameloop.go internal/gameserver/gameloop/events.go
git add internal/gameserver/gameloop/gameloop.go internal/gameserver/gameloop/events.go
git commit -m "feat(gameloop): drive auto-attack via intention + server move-to-target (fixes l2go-2za)"
```

---

## Проверка в клиенте (после Task 3)

Пересобрать GameServer (ветка `feat/intention-machine`), войти в мир, **двойным кликом** атаковать мобов на разной дистанции (близких и далёких):
- Персонаж добегает и **начинает бить сам**, без необходимости кликать на землю.
- Бой продолжается, моб умирает, EXP начисляется.
- Если моб далеко — серверное движение довозит до reach, удар стартует по прибытии.

Это чинит l2go-2za. Interact (диалог), отмена intention по клику в сторону и чистка старого approach-кода — в части 2 фазы 2 (отдельный план).

## Self-Review

- **Покрытие:** intention-состояние (Task 1), серверное движение к цели + onArrived (Task 2), перевод атаки + замена поллинга (Task 3). Interact/отмена/чистка — явно вынесены в часть 2.
- **Плейсхолдеры:** нет; весь код приведён. Места, требующие проверки фактических имён в коде (конструктор `ConnectionRegistry`, наличие `time.Now()`/`now()`), помечены явными командами проверки — это не плейсхолдеры логики, а защита от расхождения сигнатур.
- **Согласованность типов:** `Intention`, `PlayerAIState`, `setIntention`, `clearIntention`, `stopPointWithinReach`, `startMoveToTarget`, `onMovementArrived`, `beginAttackSwing`, поле `aiState` — сигнатуры единообразны между задачами. `meleeReach`/`approachTarget` — из фазы combat_range.go.
- **Известные долги (в ledger):** гонка по полям движения (тик vs handler) — теперь handler `StartMovement` всё ещё пишет `IsMoving` для движения по земле; полный single-owner и устранение гонки — отдельная задача. Хардкод скорости (l2go-tl9.1) → возможный рассинхрон прибытия; смягчён тем, что onArrived теперь серверный.
