# Intention-машина — Фаза 1: движение игрока под GameLoop (тиковая интерполяция)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Сделать GameLoop авторитетом позиции движущегося игрока: тик каждые 100мс пересчитывает позицию по серверной интерполяции, чтобы combat/interact-проверки видели свежую позицию вместо застывшей между редкими `ValidatePosition`.

**Architecture:** Чистая функция интерполяции в пакете `gameloop`; метод `advancePlayerMovement` в `tick()` двигает всех игроков с `IsMoving`; `ValidatePosition` перестаёт перезаписывать позицию во время серверного движения (становится коррекцией для стоящего игрока). Движение по-прежнему *запускается* хендлером (`StartMovement` выставляет `IsMoving`/`MoveStartPos`/`MoveDestination`/`MoveStarted`), но *продвигается* тиком.

**Tech Stack:** Go 1.22, пакеты `internal/gameserver/{gameloop,registry,usecase,models}`, тесты `go test`.

## Global Constraints

- Go 1.22.0; модуль `github.com/VerTox/l2go`.
- GameLoop — single goroutine; всё мутабельное состояние движения игрока читает/пишет только тик (кроме старта движения хендлером). Никаких новых горутин.
- Переиспользовать существующие расчёты: `usecase.CalculateMovementTime(distance float64, isRunning bool) time.Duration`.
- Линейная интерполяция (без геодаты) — как в текущем `movementUseCase`.
- Не ломать запуск движения (`MoveBackwardToLocation` → `StartMovement`) и его broadcast соседям.
- Каждый коммит — компилируется и проходит `go test ./...`.

---

## File Structure

- Create: `internal/gameserver/gameloop/movement_interp.go` — чистые функции интерполяции и шага движения игрока.
- Create: `internal/gameserver/gameloop/movement_interp_test.go` — их тесты.
- Modify: `internal/gameserver/gameloop/gameloop.go` — вызов `advancePlayerMovement` из `tick()`; сам метод.
- Modify: `internal/gameserver/handlers/client/movement.go` — `handleValidatePosition` не перезаписывает позицию во время серверного движения.

---

### Task 1: Чистые функции интерполяции позиции

**Files:**
- Create: `internal/gameserver/gameloop/movement_interp.go`
- Test: `internal/gameserver/gameloop/movement_interp_test.go`

**Interfaces:**
- Produces:
  - `func interpolatePosition(start, dest models.Position, elapsed, total time.Duration) (models.Position, bool)` — позиция вдоль прямой и `arrived`.
  - `func distanceBetween(a, b models.Position) float64` — 2D-дистанция.

- [ ] **Step 1: Написать падающий тест**

Создать `internal/gameserver/gameloop/movement_interp_test.go`:

```go
package gameloop

import (
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

func TestInterpolatePosition(t *testing.T) {
	start := models.Position{X: 0, Y: 0, Z: 0}
	dest := models.Position{X: 100, Y: 200, Z: 0}

	t.Run("halfway", func(t *testing.T) {
		pos, arrived := interpolatePosition(start, dest, 500*time.Millisecond, time.Second)
		if arrived {
			t.Fatal("halfway should not be arrived")
		}
		if pos.X != 50 || pos.Y != 100 {
			t.Errorf("got %+v, want X=50 Y=100", pos)
		}
	})

	t.Run("elapsed >= total → arrived at dest", func(t *testing.T) {
		pos, arrived := interpolatePosition(start, dest, time.Second, time.Second)
		if !arrived || pos != dest {
			t.Errorf("got pos=%+v arrived=%v, want dest arrived", pos, arrived)
		}
	})

	t.Run("total <= 0 → arrived", func(t *testing.T) {
		_, arrived := interpolatePosition(start, dest, 0, 0)
		if !arrived {
			t.Error("zero total should report arrived")
		}
	})

	t.Run("elapsed <= 0 → at start", func(t *testing.T) {
		pos, arrived := interpolatePosition(start, dest, 0, time.Second)
		if arrived || pos != start {
			t.Errorf("got pos=%+v arrived=%v, want start not-arrived", pos, arrived)
		}
	})
}

func TestDistanceBetween(t *testing.T) {
	d := distanceBetween(models.Position{X: 0, Y: 0}, models.Position{X: 3, Y: 4})
	if d != 5 {
		t.Errorf("got %g, want 5", d)
	}
}
```

- [ ] **Step 2: Запустить — убедиться, что падает**

Run: `go test ./internal/gameserver/gameloop/ -run 'TestInterpolatePosition|TestDistanceBetween' -v`
Expected: FAIL — `undefined: interpolatePosition`, `undefined: distanceBetween`.

- [ ] **Step 3: Реализовать**

Создать `internal/gameserver/gameloop/movement_interp.go`:

```go
package gameloop

import (
	"math"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// interpolatePosition returns the position along the straight line from start to
// dest after `elapsed` of the total travel time, and whether the move completed.
// Linear interpolation (no geodata), matching movementUseCase.
func interpolatePosition(start, dest models.Position, elapsed, total time.Duration) (models.Position, bool) {
	if total <= 0 || elapsed >= total {
		return dest, true
	}
	if elapsed <= 0 {
		return start, false
	}
	progress := float64(elapsed) / float64(total)
	return models.Position{
		X: start.X + int(float64(dest.X-start.X)*progress),
		Y: start.Y + int(float64(dest.Y-start.Y)*progress),
		Z: start.Z + int(float64(dest.Z-start.Z)*progress),
	}, false
}

// distanceBetween is the 2D distance between two positions.
func distanceBetween(a, b models.Position) float64 {
	dx := float64(b.X - a.X)
	dy := float64(b.Y - a.Y)
	return math.Sqrt(dx*dx + dy*dy)
}
```

- [ ] **Step 4: Запустить — убедиться, что проходит**

Run: `go test ./internal/gameserver/gameloop/ -run 'TestInterpolatePosition|TestDistanceBetween' -v`
Expected: PASS (все подтесты).

- [ ] **Step 5: Коммит**

```bash
git add internal/gameserver/gameloop/movement_interp.go internal/gameserver/gameloop/movement_interp_test.go
git commit -m "feat(gameloop): pure position interpolation helpers for tick-driven movement"
```

---

### Task 2: Тиковое продвижение движения игроков

**Files:**
- Modify: `internal/gameserver/gameloop/movement_interp.go` (добавить `stepPlayerMovement`)
- Modify: `internal/gameserver/gameloop/gameloop.go` (метод `advancePlayerMovement`, вызов в `tick()`)
- Test: `internal/gameserver/gameloop/movement_interp_test.go`

**Interfaces:**
- Consumes: `interpolatePosition`, `distanceBetween` (Task 1); `usecase.CalculateMovementTime`; `registry.PlayerWorldState` поля `IsMoving, IsRunning, MoveStartPos, MoveDestination, MoveStarted, Heading`; `gl.world.GetAllPlayers()`, `gl.world.UpdatePlayerPosition(ctx, charID, pos, heading)`.
- Produces:
  - `func stepPlayerMovement(player *registry.PlayerWorldState, now time.Time) (models.Position, bool)` — позиция игрока на момент `now` и `arrived`.
  - `func (gl *GameLoop) advancePlayerMovement(now time.Time)` — двигает всех `IsMoving` игроков.

- [ ] **Step 1: Написать падающий тест**

Добавить в `internal/gameserver/gameloop/movement_interp_test.go`:

```go
import "github.com/VerTox/l2go/internal/gameserver/registry"

func TestStepPlayerMovement(t *testing.T) {
	t.Run("far past start → arrived at destination", func(t *testing.T) {
		player := &registry.PlayerWorldState{
			IsMoving:        true,
			IsRunning:       true,
			MoveStartPos:    models.Position{X: 0, Y: 0, Z: 0},
			MoveDestination: models.Position{X: 1000, Y: 0, Z: 0},
			MoveStarted:     time.Now().Add(-1 * time.Hour), // заведомо дошёл
		}
		pos, arrived := stepPlayerMovement(player, time.Now())
		if !arrived || pos != player.MoveDestination {
			t.Errorf("got pos=%+v arrived=%v, want dest arrived", pos, arrived)
		}
	})

	t.Run("just started → near start, not arrived", func(t *testing.T) {
		now := time.Now()
		player := &registry.PlayerWorldState{
			IsMoving:        true,
			IsRunning:       true,
			MoveStartPos:    models.Position{X: 0, Y: 0, Z: 0},
			MoveDestination: models.Position{X: 1000, Y: 0, Z: 0},
			MoveStarted:     now,
		}
		pos, arrived := stepPlayerMovement(player, now)
		if arrived {
			t.Fatal("just-started move should not be arrived")
		}
		if pos.X > 10 {
			t.Errorf("got X=%d, want near start (0)", pos.X)
		}
	})
}
```

- [ ] **Step 2: Запустить — убедиться, что падает**

Run: `go test ./internal/gameserver/gameloop/ -run TestStepPlayerMovement -v`
Expected: FAIL — `undefined: stepPlayerMovement`.

- [ ] **Step 3: Реализовать `stepPlayerMovement`**

Добавить в `internal/gameserver/gameloop/movement_interp.go` (импорты `registry`, `usecase`):

```go
// stepPlayerMovement computes a moving player's position at time `now` via
// server-side interpolation, and whether the move has completed.
func stepPlayerMovement(player *registry.PlayerWorldState, now time.Time) (models.Position, bool) {
	distance := distanceBetween(player.MoveStartPos, player.MoveDestination)
	total := usecase.CalculateMovementTime(distance, player.IsRunning)
	return interpolatePosition(player.MoveStartPos, player.MoveDestination, now.Sub(player.MoveStarted), total)
}
```

Добавить в блок импортов файла:
```go
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
```

- [ ] **Step 4: Запустить — убедиться, что проходит**

Run: `go test ./internal/gameserver/gameloop/ -run TestStepPlayerMovement -v`
Expected: PASS.

- [ ] **Step 5: Реализовать `advancePlayerMovement` и вызвать из тика**

Добавить метод в `internal/gameserver/gameloop/gameloop.go` (рядом с `tick`):

```go
// advancePlayerMovement moves every in-progress player along its path using
// server-side interpolation, so the loop and combat checks always see a fresh
// position instead of the stale value between client ValidatePosition packets.
func (gl *GameLoop) advancePlayerMovement(now time.Time) {
	for charID, player := range gl.world.GetAllPlayers() {
		if !player.IsMoving {
			continue
		}
		pos, arrived := stepPlayerMovement(player, now)
		_ = gl.world.UpdatePlayerPosition(context.Background(), charID, pos, player.Heading)
		if arrived {
			player.IsMoving = false
			player.MoveStartPos = models.Position{}
			player.MoveDestination = models.Position{}
		}
	}
}
```

В функции `tick()` после метки `executeEvents:` добавить вызов перед циклом событий:

```go
executeEvents:
	// Execute all events whose time has come
	now := time.Now()
	gl.advancePlayerMovement(now)
	for {
		e := gl.events.Peek()
		...
	}
```

(Файл уже импортирует `context`, `time`, `models` — новые импорты не нужны.)

- [ ] **Step 6: Запустить весь пакет и сборку**

Run: `go build ./... && go test ./internal/gameserver/gameloop/ -v`
Expected: BUILD OK, PASS.

- [ ] **Step 7: Коммит**

```bash
gofmt -w internal/gameserver/gameloop/movement_interp.go internal/gameserver/gameloop/gameloop.go
git add internal/gameserver/gameloop/movement_interp.go internal/gameserver/gameloop/movement_interp_test.go internal/gameserver/gameloop/gameloop.go
git commit -m "feat(gameloop): advance player movement each tick via server interpolation"
```

---

### Task 3: `ValidatePosition` не перезаписывает позицию во время серверного движения

**Files:**
- Modify: `internal/gameserver/handlers/client/movement.go:209-213` (`handleValidatePosition`)

**Interfaces:**
- Consumes: `playerState.IsMoving` (registry); существующий `h.movementUseCase.UpdatePosition`.
- Produces: поведение — при `IsMoving` позиция игрока не перезаписывается клиентским пакетом (авторитет у тика); коррекция дрейфа (`NeedsCorrection`) и уведомление региона сохраняются.

**Контекст:** сейчас `handleValidatePosition` после проверки **всегда** вызывает `UpdatePosition(clientPos)` (строка 210), затирая интерполированную тиком позицию и возвращая «застывшее» поведение. Во время серверного движения позицией владеет тик; клиентский `ValidatePosition` используем только для анти-чит-коррекции и трекинга региона.

- [ ] **Step 1: Прочитать текущий блок**

Open: `internal/gameserver/handlers/client/movement.go:194-228`. Убедиться, что блок соответствует шагу 2 (строки могли сдвинуться).

- [ ] **Step 2: Заменить безусловный `UpdatePosition` на условный**

Заменить:

```go
	// Update position if client is close enough
	if err := h.movementUseCase.UpdatePosition(ctx, playerState.CharID, clientPos, validatePacket.Heading); err != nil {
		logger.Warn().Err(err).Msg("failed to update position")
		// Don't fail validation for position update errors
	}
```

на:

```go
	// While the server is interpolating this player's movement (tick is the
	// position authority), do NOT overwrite the position with the client packet —
	// that reintroduced the stale-position bug. Only sync the client position when
	// the player is standing still. Drift correction above still applies.
	if !playerState.IsMoving {
		if err := h.movementUseCase.UpdatePosition(ctx, playerState.CharID, clientPos, validatePacket.Heading); err != nil {
			logger.Warn().Err(err).Msg("failed to update position")
			// Don't fail validation for position update errors
		}
	}
```

- [ ] **Step 3: Сборка**

Run: `go build ./...`
Expected: BUILD OK.

- [ ] **Step 4: Полный прогон тестов**

Run: `go test ./...`
Expected: все пакеты `ok` (или `no test files`).

- [ ] **Step 5: Коммит**

```bash
gofmt -w internal/gameserver/handlers/client/movement.go
git add internal/gameserver/handlers/client/movement.go
git commit -m "fix(gameserver): let tick own moving player's position, ValidatePosition only corrects when idle"
```

---

## Проверка в клиенте (после Task 3)

Пересобрать GameServer, войти в мир, побегать по карте. Ожидается:
- Персонаж движется плавно, без рывков назад (тик и клиент согласованы).
- В логах при движении позиция игрока меняется каждый тик (не застывает на 5с).
- Существующее движение соседей (broadcast) по-прежнему работает.

Это фундамент. Атака/интеракция чинятся в **фазе 2** (intention-ядро) — отдельным планом после проверки фазы 1.

## Self-Review

- **Покрытие spec (фаза 1 «движение под GameLoop»):** тиковая интерполяция — Task 1+2; `ValidatePosition` → коррекция — Task 3. ✓
- **Плейсхолдеры:** нет — весь код приведён. ✓
- **Согласованность типов:** `interpolatePosition`/`distanceBetween`/`stepPlayerMovement`/`advancePlayerMovement` — сигнатуры совпадают между задачами; `CalculateMovementTime`, `GetAllPlayers`, `UpdatePlayerPosition` — проверены в коде. ✓
- **Вне объёма:** intention-машина, attack/interact-переходы, заготовки CAST/FOLLOW — фаза 2; чистка старого approach-поллинга — фаза 3.
