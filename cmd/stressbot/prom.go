package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// This file queries the game server's Prometheus tick-health metrics
// (l2go_gameloop_*, exposed on :2112 and scraped into the l2go-prometheus
// container) so a scenario run prints one combined report instead of the old
// two-source stitch of fleet stats + log-scraped tick health.

// promMetric is one line of the tick-health summary. expr uses %s for the window
// (a Prometheus duration like "180s") so the peak is taken over exactly the run.
type promMetric struct {
	label string
	expr  string
	unit  string // "ms" (seconds→ms), "" (raw count)
}

var promMetrics = []promMetric{
	{"tick work p99 (peak)", `max_over_time((histogram_quantile(0.99, sum(rate(l2go_gameloop_tick_work_seconds_bucket[15s])) by (le)))[%s:5s])`, "ms"},
	{"tick gap p99 (peak)", `max_over_time((histogram_quantile(0.99, sum(rate(l2go_gameloop_tick_gap_seconds_bucket[15s])) by (le)))[%s:5s])`, "ms"},
	{"behind ticks (total)", `increase(l2go_gameloop_tick_behind_total[%s])`, ""},
	{"players (peak)", `max_over_time(l2go_gameloop_players[%s:5s])`, ""},
	{"command backlog (peak)", `max_over_time(l2go_gameloop_command_backlog[%s:5s])`, ""},
	{"goroutines (peak)", `max_over_time(go_goroutines{job="gameserver"}[%s:5s])`, ""},
	{"GC time total", `increase(go_gc_duration_seconds_sum{job="gameserver"}[%s])`, "ms"},
}

// promQueryScalar runs an instant query at time `at` and returns the first value.
// ok=false means the query matched no series (metric absent).
func promQueryScalar(base, expr string, at time.Time) (float64, bool, error) {
	u := fmt.Sprintf("%s/api/v1/query?query=%s&time=%d",
		base, url.QueryEscape(expr), at.Unix())
	resp, err := http.Get(u)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()

	var out struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string            `json:"resultType"`
			Result     []json.RawMessage `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, false, err
	}
	if out.Status != "success" || len(out.Data.Result) == 0 {
		return 0, false, nil
	}
	// vector element: {"metric":{...},"value":[ts,"val"]}
	var elem struct {
		Value []any `json:"value"`
	}
	if err := json.Unmarshal(out.Data.Result[0], &elem); err != nil || len(elem.Value) < 2 {
		return 0, false, nil
	}
	s, ok := elem.Value[1].(string)
	if !ok {
		return 0, false, nil
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%g", &f); err != nil {
		return 0, false, nil
	}
	return f, true, nil
}

// printPromSummary queries the tick-health peaks over [start,end] and prints them.
// Non-fatal: if Prometheus is unreachable or the series are absent (server not
// rebuilt with the prom endpoint yet), it says so and returns.
func printPromSummary(base string, start, end time.Time) {
	window := fmt.Sprintf("%ds", int(end.Sub(start).Seconds())+1)
	fmt.Printf("\n── game-loop tick health over the run (Prometheus %s, window %s) ──\n", base, window)

	any := false
	for _, m := range promMetrics {
		v, ok, err := promQueryScalar(base, fmt.Sprintf(m.expr, window), end)
		if err != nil {
			fmt.Printf("  %-24s Prometheus unavailable: %v\n", m.label, err)
			return
		}
		if !ok {
			continue
		}
		any = true
		if m.unit == "ms" {
			fmt.Printf("  %-24s %.0f ms\n", m.label, v*1000)
		} else {
			fmt.Printf("  %-24s %.0f\n", m.label, v)
		}
	}
	if !any {
		fmt.Printf("  no l2go_gameloop_* series yet — gameserver not rebuilt with the :2112 endpoint, or target down\n")
	}
}
