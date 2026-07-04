package gameloop

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PromMetrics mirrors the loop's per-tick health samples into Prometheus
// collectors so a scraper can plot tick work/gap/backlog against player count
// over time — the time-series view the 10s log report can't give (l2go-5pc).
//
// It owns a private registry so the /metrics endpoint exposes exactly these
// series plus Go runtime metrics, with no coupling to the global default
// registry (keeps tests isolated). Every collector is internally goroutine-safe:
// the loop goroutine writes via recordTick/setPlayers while the scrape goroutine
// reads — so this does not break the loop's single-writer ownership model.
type PromMetrics struct {
	reg        *prometheus.Registry
	workSecs   prometheus.Histogram
	gapSecs    prometheus.Histogram
	phaseSecs  *prometheus.HistogramVec
	knownPlayers    prometheus.Histogram
	knownPlayersMax prometheus.Gauge
	behind        prometheus.Counter
	players       prometheus.Gauge
	cmdBacklog    prometheus.Gauge
	activeRegions prometheus.Gauge
	npcTotal      prometheus.Gauge
	worldEntries  *prometheus.CounterVec
	entryDuration prometheus.Histogram
}

// tickWorkBuckets resolve per-tick work time from well under a millisecond, past
// the 100ms deadline, out to ~1s — the span seen from idle to heavy load.
var tickWorkBuckets = []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1}

// tickGapBuckets resolve the scheduling gap tightly around the 100ms cadence and
// the 150ms behind-threshold, then out to whole seconds under a stall.
var tickGapBuckets = []float64{0.05, 0.075, 0.1, 0.125, 0.15, 0.2, 0.3, 0.5, 1, 2}

// fanOutBuckets resolve a player's visible-player set from empty up to the dense
// clustering that drives the O(N²) visibility cost (hundreds-to-thousands in one spot).
var fanOutBuckets = []float64{0, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000}

// entryDurationBuckets resolve world-entry handler latency from a few ms up to
// several seconds — under mass entry the DB-bound work (skill reconcile, item
// list) is what stretches, exactly the 0ar failed-entry symptom.
var entryDurationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

// NewPromMetrics builds the collectors on a fresh private registry.
func NewPromMetrics() *PromMetrics {
	reg := prometheus.NewRegistry()
	pm := &PromMetrics{
		reg: reg,
		workSecs: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "l2go_gameloop_tick_work_seconds",
			Help:    "Time the game loop spent processing a single tick.",
			Buckets: tickWorkBuckets,
		}),
		gapSecs: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "l2go_gameloop_tick_gap_seconds",
			Help:    "Wall-clock interval between successive tick starts (scheduling health).",
			Buckets: tickGapBuckets,
		}),
		phaseSecs: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "l2go_gameloop_tick_phase_seconds",
			Help:    "Time spent in each tick subsystem (core/regen/buffs/region_cleanup/autosave) — attributes the tick budget.",
			Buckets: tickWorkBuckets,
		}, []string{"phase"}),
		knownPlayers: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "l2go_gameloop_known_players",
			Help:    "Per-player count of other players spawned to them (visibility fan-out) — the O(N²) driver.",
			Buckets: fanOutBuckets,
		}),
		knownPlayersMax: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "l2go_gameloop_known_players_max",
			Help: "Largest visibility fan-out seen in the last sampling window (worst-case local density).",
		}),
		behind: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "l2go_gameloop_tick_behind_total",
			Help: "Ticks whose scheduling gap exceeded the behind threshold (missed cadence).",
		}),
		players: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "l2go_gameloop_players",
			Help: "Online players observed by the game loop.",
		}),
		cmdBacklog: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "l2go_gameloop_command_backlog",
			Help: "Command-channel depth seen at the start of a tick (backpressure).",
		}),
		activeRegions: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "l2go_gameloop_active_regions",
			Help: "Region-grid cells currently active (have players nearby) — the loop's working set.",
		}),
		npcTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "l2go_gameloop_npc_total",
			Help: "Total NPC instances registered in the world.",
		}),
		worldEntries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "l2go_world_entry_total",
			Help: "World-entry attempts by outcome (ok / player_not_found / no_session / send_failed).",
		}, []string{"result"}),
		entryDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "l2go_world_entry_duration_seconds",
			Help:    "Server-side world-entry handler processing time (skill reconcile, packet build/send).",
			Buckets: entryDurationBuckets,
		}),
	}
	reg.MustRegister(pm.workSecs, pm.gapSecs, pm.phaseSecs, pm.knownPlayers, pm.knownPlayersMax, pm.behind, pm.players, pm.cmdBacklog, pm.activeRegions, pm.npcTotal, pm.worldEntries, pm.entryDuration)
	// Go runtime metrics (goroutines/GC/heap) — cheap and valuable under load, where
	// goroutine-per-connection growth and GC pauses matter as much as tick time.
	reg.MustRegister(collectors.NewGoCollector())
	return pm
}

// recordTick feeds one tick's samples to the collectors. Called on the loop
// goroutine every tick; nil-safe so a loop without Prometheus wiring is a no-op.
func (pm *PromMetrics) recordTick(gap, work time.Duration, cmdDepth int, behind bool) {
	if pm == nil {
		return
	}
	pm.workSecs.Observe(work.Seconds())
	pm.gapSecs.Observe(gap.Seconds())
	pm.cmdBacklog.Set(float64(cmdDepth))
	if behind {
		pm.behind.Inc()
	}
}

// observePhase records how long one tick subsystem took, keyed by phase. Called
// on the loop goroutine; nil-safe. Attributes the aggregate tick work so a
// spike (e.g. region cleanup over ~38K NPCs) is localised rather than hidden in
// the total.
func (pm *PromMetrics) observePhase(phase string, d time.Duration) {
	if pm == nil {
		return
	}
	pm.phaseSecs.WithLabelValues(phase).Observe(d.Seconds())
}

// setPlayers updates the online-players gauge. Called on the loop goroutine.
func (pm *PromMetrics) setPlayers(n int) {
	if pm == nil {
		return
	}
	pm.players.Set(float64(n))
}

// RegisterQueueDepth registers a gauge that reports the current length of an
// async queue, read via length() at scrape time — no sampler goroutine, always
// fresh. Used for the persistence sinks (save/recharge/learn) whose backlog is
// an early warning that DB latency is stalling the loop under load. length() is
// called on the scrape goroutine; len(chan) is safe to read concurrently.
// nil-safe so a loop without Prometheus wiring is a no-op.
func (pm *PromMetrics) RegisterQueueDepth(name, help string, length func() int) {
	if pm == nil {
		return
	}
	pm.reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, func() float64 { return float64(length()) }))
}

// observeKnownPlayers records one player's visibility fan-out (count of other
// players spawned to them). Called on the loop goroutine while sweeping the
// player set; nil-safe. KnownPlayers is loop-owned so this read is race-free.
func (pm *PromMetrics) observeKnownPlayers(n int) {
	if pm == nil {
		return
	}
	pm.knownPlayers.Observe(float64(n))
}

// setKnownPlayersMax records the peak fan-out seen in the sampling window.
func (pm *PromMetrics) setKnownPlayersMax(n int) {
	if pm == nil {
		return
	}
	pm.knownPlayersMax.Set(float64(n))
}

// setWorldInventory updates the world-size gauges (active region-grid cells and
// total NPC count). Called on the loop goroutine from the report window, so the
// loop-owned activeRegions map is read race-free. nil-safe.
func (pm *PromMetrics) setWorldInventory(activeRegions, npcTotal int) {
	if pm == nil {
		return
	}
	pm.activeRegions.Set(float64(activeRegions))
	pm.npcTotal.Set(float64(npcTotal))
}

// RecordWorldEntry records one world-entry attempt: its outcome (bounded result
// enum) and how long the handler took. Called from the connection goroutine —
// the counter/histogram are internally goroutine-safe. nil-safe.
func (pm *PromMetrics) RecordWorldEntry(result string, dur time.Duration) {
	if pm == nil {
		return
	}
	pm.worldEntries.WithLabelValues(result).Inc()
	pm.entryDuration.Observe(dur.Seconds())
}

// Handler serves the Prometheus text exposition for these collectors.
func (pm *PromMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(pm.reg, promhttp.HandlerOpts{})
}
