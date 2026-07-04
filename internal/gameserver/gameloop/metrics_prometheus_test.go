package gameloop

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestPromMetricsRecordTick verifies per-tick samples land in the right
// collectors: histograms count every observation, the behind counter only
// increments for behind ticks, and the gauges hold the latest value.
func TestPromMetricsRecordTick(t *testing.T) {
	pm := NewPromMetrics()

	// Two healthy ticks, one behind schedule (gap over threshold).
	pm.recordTick(90*time.Millisecond, 5*time.Millisecond, 0, false)
	pm.recordTick(100*time.Millisecond, 8*time.Millisecond, 2, false)
	pm.recordTick(300*time.Millisecond, 120*time.Millisecond, 7, true)

	pm.setPlayers(42)

	if got := histogramSampleCount(t, pm, "l2go_gameloop_tick_work_seconds"); got != 3 {
		t.Errorf("work histogram sample count = %d, want 3", got)
	}
	if got := histogramSampleCount(t, pm, "l2go_gameloop_tick_gap_seconds"); got != 3 {
		t.Errorf("gap histogram sample count = %d, want 3", got)
	}
	if got := testutil.ToFloat64(pm.behind); got != 1 {
		t.Errorf("behind counter = %v, want 1", got)
	}
	if got := testutil.ToFloat64(pm.players); got != 42 {
		t.Errorf("players gauge = %v, want 42", got)
	}
	if got := testutil.ToFloat64(pm.cmdBacklog); got != 7 {
		t.Errorf("command backlog gauge = %v, want 7 (last set)", got)
	}
}

// TestPromMetricsHandlerExposition verifies the /metrics endpoint serves the
// text exposition naming every tick-health series.
func TestPromMetricsHandlerExposition(t *testing.T) {
	pm := NewPromMetrics()
	pm.recordTick(100*time.Millisecond, 10*time.Millisecond, 1, false)
	pm.setPlayers(5)

	rec := httptest.NewRecorder()
	pm.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))

	body := rec.Body.String()
	for _, name := range []string{
		"l2go_gameloop_tick_work_seconds",
		"l2go_gameloop_tick_gap_seconds",
		"l2go_gameloop_tick_behind_total",
		"l2go_gameloop_players",
		"l2go_gameloop_command_backlog",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("exposition missing metric %q", name)
		}
	}
}

// TestPromMetricsNilSafe verifies a loop without Prometheus wiring (nil sink)
// treats every update as a no-op instead of panicking.
func TestPromMetricsNilSafe(t *testing.T) {
	var pm *PromMetrics
	pm.recordTick(100*time.Millisecond, 10*time.Millisecond, 3, true)
	pm.setPlayers(9)
	pm.RegisterQueueDepth("l2go_test_nil", "no-op", func() int { return 1 })
}

// TestPromMetricsQueueDepthGauge verifies a registered queue-depth gauge reads
// the live channel length at scrape time (no sampler goroutine needed).
func TestPromMetricsQueueDepthGauge(t *testing.T) {
	pm := NewPromMetrics()
	ch := make(chan int, 8)
	pm.RegisterQueueDepth("l2go_sink_save_queue_depth", "test sink depth", func() int { return len(ch) })

	if got := gaugeValue(t, pm, "l2go_sink_save_queue_depth"); got != 0 {
		t.Errorf("empty channel depth = %v, want 0", got)
	}

	ch <- 1
	ch <- 2
	ch <- 3
	if got := gaugeValue(t, pm, "l2go_sink_save_queue_depth"); got != 3 {
		t.Errorf("depth after 3 enqueues = %v, want 3 (must reflect live len at scrape)", got)
	}

	<-ch
	if got := gaugeValue(t, pm, "l2go_sink_save_queue_depth"); got != 2 {
		t.Errorf("depth after 1 dequeue = %v, want 2", got)
	}
}

// gaugeValue gathers the named gauge from the metrics' registry and returns its
// current value.
func gaugeValue(t *testing.T, pm *PromMetrics, name string) float64 {
	t.Helper()
	families, err := pm.reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range families {
		if mf.GetName() != name {
			continue
		}
		metrics := mf.GetMetric()
		if len(metrics) == 0 {
			t.Fatalf("no metric samples for %q", name)
		}
		return metrics[0].GetGauge().GetValue()
	}
	t.Fatalf("metric family %q not found", name)
	return 0
}

// histogramSampleCount gathers the named histogram from the metrics' registry
// and returns its total observation count.
func histogramSampleCount(t *testing.T, pm *PromMetrics, name string) uint64 {
	t.Helper()
	families, err := pm.reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range families {
		if mf.GetName() != name {
			continue
		}
		metrics := mf.GetMetric()
		if len(metrics) == 0 {
			t.Fatalf("no metric samples for %q", name)
		}
		return metrics[0].GetHistogram().GetSampleCount()
	}
	t.Fatalf("metric family %q not found", name)
	return 0
}
