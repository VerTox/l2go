#!/usr/bin/env bash
# Run one clustered-vs-spread load scenario (see cmd/stressbot/SCENARIOS.md) and
# print ONLY a compact result block — the noisy per-tick progress stays out.
# Designed to be invoked by a scenario-runner subagent so heavy run output never
# reaches the orchestrator's context.
#
# Usage: scripts/loadscenario.sh <A|B|C|D|E|F> [hold_seconds]
#   A=1k clustered  B=5k clustered  C=10k clustered  D=1k spread(5×200)
#   E=5k spread(5×1000, dense towns)  F=5k grid(25×200, low local density)
#
# Env overrides: CONC (entry concurrency, def 60), WALK (def 2s), TALK (def 5s).
set -euo pipefail
cd "$(dirname "$0")/.."

sc="${1:?usage: loadscenario.sh <A|B|C|D|E> [hold_seconds]}"
hold="${2:-45}"
conc="${CONC:-60}"
walk="${WALK:-2s}"
talk="${TALK:-5s}"

case "$sc" in
  A) n=1000;  prefix=clust  ;;
  B) n=5000;  prefix=clust  ;;
  C) n=10000; prefix=clust  ;;
  D) n=1000;  prefix=spread ;;
  E) n=5000;  prefix=spread ;;
  F) n=5000;  prefix=grid   ;;  # 25 points x 200 — low local density at 5k scale
  *) echo "unknown scenario '$sc' (want A|B|C|D|E|F)"; exit 2 ;;
esac

bin="$(mktemp -d)/stressbot"
go build -o "$bin" ./cmd/stressbot

restarts_before="$(docker inspect l2go-gameserver --format '{{.RestartCount}}' 2>/dev/null || echo '?')"

out="$(mktemp)"
"$bin" -n "$n" -c "$conc" -prefix "$prefix" -enter -walk "$walk" -talk "$talk" -hold "${hold}s" >"$out" 2>&1 || true

restarts_after="$(docker inspect l2go-gameserver --format '{{.RestartCount}}' 2>/dev/null || echo '?')"

echo "=== SCENARIO $sc : $n bots, prefix=$prefix, hold=${hold}s, walk=$walk talk=$talk ==="
grep -E "^ramp done|^entry latency" "$out" || echo "(no ramp line — run may have failed; last lines:)"
grep -E "tick work|tick gap|behind ticks|players \(peak\)|command backlog|goroutines|GC time" "$out" \
  || { echo "(no Prometheus summary)"; tail -3 "$out"; }
echo "server restarts: ${restarts_before} -> ${restarts_after} (crash if it increased)"

rm -f "$out"
