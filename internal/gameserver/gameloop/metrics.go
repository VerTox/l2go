package gameloop

import (
	"sort"
	"time"

	"github.com/rs/zerolog/log"
)

// tickMetricsReportInterval is how often the loop emits an aggregated tick-health
// report. Kept coarse so instrumentation never becomes its own load.
const tickMetricsReportInterval = 10 * time.Second

// behindThreshold marks a tick whose scheduling gap indicates the loop missed a
// beat (couldn't keep the 100ms cadence). 1.5× the interval: a normal tick lands
// near tickInterval; anything past 150ms means work spilled over the deadline.
const behindThreshold = tickInterval + tickInterval/2

// tickMetrics accumulates per-tick samples over a reporting window. Owned by the
// loop goroutine only — no synchronisation needed.
type tickMetrics struct {
	windowStart time.Time
	gaps        []time.Duration // interval between successive tick starts (scheduling health)
	works       []time.Duration // time spent doing tick work (CPU per tick)
	maxCmdDepth int             // deepest command-channel backlog seen (backpressure)
	behindTicks int             // ticks whose gap exceeded behindThreshold
}

func newTickMetrics(now time.Time) *tickMetrics {
	return &tickMetrics{windowStart: now}
}

// record adds one tick's samples. gap is the wall-clock interval since the
// previous tick start, work is how long the tick's processing took, cmdDepth is
// the command-channel length observed at the start of the tick.
func (m *tickMetrics) record(gap, work time.Duration, cmdDepth int) {
	m.gaps = append(m.gaps, gap)
	m.works = append(m.works, work)
	if cmdDepth > m.maxCmdDepth {
		m.maxCmdDepth = cmdDepth
	}
	if gap > behindThreshold {
		m.behindTicks++
	}
}

// tickSummary is the aggregated view of a reporting window.
type tickSummary struct {
	Window        time.Duration
	Ticks         int
	ExpectedTicks int
	GapP50        time.Duration
	GapP99        time.Duration
	GapMax        time.Duration
	WorkP50       time.Duration
	WorkP99       time.Duration
	WorkMax       time.Duration
	MaxCmdDepth   int
	BehindTicks   int
	Players       int
}

func (m *tickMetrics) summary(now time.Time, players int) tickSummary {
	window := now.Sub(m.windowStart)
	expected := 0
	if window > 0 {
		expected = int(window / tickInterval)
	}
	return tickSummary{
		Window:        window,
		Ticks:         len(m.gaps),
		ExpectedTicks: expected,
		GapP50:        percentileDuration(m.gaps, 0.50),
		GapP99:        percentileDuration(m.gaps, 0.99),
		GapMax:        percentileDuration(m.gaps, 1.0),
		WorkP50:       percentileDuration(m.works, 0.50),
		WorkP99:       percentileDuration(m.works, 0.99),
		WorkMax:       percentileDuration(m.works, 1.0),
		MaxCmdDepth:   m.maxCmdDepth,
		BehindTicks:   m.behindTicks,
		Players:       players,
	}
}

// reset clears the window for the next reporting interval.
func (m *tickMetrics) reset(now time.Time) {
	m.windowStart = now
	m.gaps = m.gaps[:0]
	m.works = m.works[:0]
	m.maxCmdDepth = 0
	m.behindTicks = 0
}

// report logs one window's aggregated tick health at info level.
func (m *tickMetrics) report(now time.Time, players int) {
	s := m.summary(now, players)
	log.Info().
		Int("players", s.Players).
		Int("ticks", s.Ticks).
		Int("expected_ticks", s.ExpectedTicks).
		Int("behind_ticks", s.BehindTicks).
		Int("max_cmd_depth", s.MaxCmdDepth).
		Dur("gap_p50", s.GapP50).
		Dur("gap_p99", s.GapP99).
		Dur("gap_max", s.GapMax).
		Dur("work_p50", s.WorkP50).
		Dur("work_p99", s.WorkP99).
		Dur("work_max", s.WorkMax).
		Msg("game loop tick health")
}

// percentileDuration returns the p-quantile (0..1) of samples without mutating
// the caller's slice. Nearest-rank on a sorted copy; returns 0 for an empty set.
func percentileDuration(samples []time.Duration, p float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	// Nearest-rank: rank = ceil(p*N), 1-indexed.
	rank := int(p*float64(len(sorted)) + 0.999999)
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}
