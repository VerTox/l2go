package gameloop

import (
	"testing"
	"time"
)

func TestPercentileDuration(t *testing.T) {
	samples := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	tests := []struct {
		p    float64
		want time.Duration
	}{
		{0.50, 50 * time.Millisecond},
		{0.99, 100 * time.Millisecond},
		{1.0, 100 * time.Millisecond},
	}
	for _, tt := range tests {
		got := percentileDuration(samples, tt.p)
		if got != tt.want {
			t.Errorf("percentileDuration(p=%.2f) = %v, want %v", tt.p, got, tt.want)
		}
	}

	// percentileDuration must not mutate the caller's slice ordering.
	shuffled := []time.Duration{50 * time.Millisecond, 10 * time.Millisecond, 90 * time.Millisecond}
	_ = percentileDuration(shuffled, 0.5)
	if shuffled[0] != 50*time.Millisecond {
		t.Errorf("percentileDuration mutated the input slice: %v", shuffled)
	}

	if got := percentileDuration(nil, 0.5); got != 0 {
		t.Errorf("percentileDuration(nil) = %v, want 0", got)
	}
}

func TestTickMetricsSummary(t *testing.T) {
	start := time.Unix(1000, 0)
	m := newTickMetrics(start)

	// 100 ticks: 98 healthy (~100ms gap) and 2 that fell behind (250ms gap).
	for i := 0; i < 98; i++ {
		m.record(100*time.Millisecond, 5*time.Millisecond, 3)
	}
	m.record(250*time.Millisecond, 40*time.Millisecond, 700)
	m.record(250*time.Millisecond, 60*time.Millisecond, 900)

	// Window of ~10s → expected 100 ticks at the 100ms interval.
	now := start.Add(10 * time.Second)
	s := m.summary(now, 100)

	if s.Ticks != 100 {
		t.Errorf("Ticks = %d, want 100", s.Ticks)
	}
	if s.ExpectedTicks != 100 {
		t.Errorf("ExpectedTicks = %d, want 100", s.ExpectedTicks)
	}
	if s.BehindTicks != 2 {
		t.Errorf("BehindTicks = %d, want 2 (gaps over the behind threshold)", s.BehindTicks)
	}
	if s.MaxCmdDepth != 900 {
		t.Errorf("MaxCmdDepth = %d, want 900", s.MaxCmdDepth)
	}
	if s.WorkMax != 60*time.Millisecond {
		t.Errorf("WorkMax = %v, want 60ms", s.WorkMax)
	}
	if s.GapMax != 250*time.Millisecond {
		t.Errorf("GapMax = %v, want 250ms", s.GapMax)
	}
	if s.Players != 100 {
		t.Errorf("Players = %d, want 100", s.Players)
	}

	// After reset the window is empty again.
	m.reset(now)
	empty := m.summary(now.Add(time.Second), 0)
	if empty.Ticks != 0 {
		t.Errorf("after reset Ticks = %d, want 0", empty.Ticks)
	}
}
