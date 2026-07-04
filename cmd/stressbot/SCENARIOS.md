# Load-test scenarios: clustered vs spread

Goal: compare game-server performance at the **same total bot count** when players
are **clustered in one point** vs **spread across 5 towns**. This isolates the
O(N²) visibility cost on the game loop (see beads `l2go-cwj`): clustered → every
bot sees every other bot; spread → each town only sees its ~N/5 locals, so the
visibility work drops ~5×.

All bots use the native Go client (`cmd/stressbot`, goroutine-per-bot) and
`-walk`/`-talk` so movement/chat actually exercises broadcast + visibility, not
just idle presence.

## Towns (High Five, from `references/data/mapregion/*`)

| Town   | X       | Y      | Z     |
|--------|---------|--------|-------|
| Giran  | 82480   | 149087 | -3350 |
| Aden   | 146494  | 30584  | -2420 |
| Dion   | 19025   | 145245 | -3107 |
| Gludio | -14288  | 122752 | -3000 |
| Oren   | 80304   | 56241  | -1500 |

Spread string (pass to `seedbots -towns`):
```
82480,149087,-3350;146494,30584,-2420;19025,145245,-3107;-14288,122752,-3000;80304,56241,-1500
```

## Seed the character sets (idempotent; accounts auto-create on first login)

```bash
# Clustered: everyone at Giran (single point). Supports the 1k / 5k / 10k runs.
go run ./cmd/seedbots -n 10000 -prefix clust -towns "82480,149087,-3350"

# Spread: round-robin across the 5 towns. Supports 1k (5×200) and 5k (5×1000).
go run ./cmd/seedbots -n 5000 -prefix spread \
  -towns "82480,149087,-3350;146494,30584,-2420;19025,145245,-3107;-14288,122752,-3000;80304,56241,-1500"
```

`seedbots` distributes round-robin by account index, so `spread` gives an even
split: at `-n 1000` → 200/town, at `-n 5000` → 1000/town.

## Run the scenarios

Common flags: `-enter -c 60 -walk 2s -talk 5s -hold 45s -gs 127.0.0.1:7777`.

| # | Scenario         | Command                                                        |
|---|------------------|----------------------------------------------------------------|
| A | 1k clustered     | `stressbot -prefix clust  -n 1000  -enter -c 60 -walk 2s -talk 5s -hold 45s` |
| B | 5k clustered     | `stressbot -prefix clust  -n 5000  -enter -c 60 -walk 2s -talk 5s -hold 45s` |
| C | 10k clustered    | `stressbot -prefix clust  -n 10000 -enter -c 60 -walk 2s -talk 5s -hold 45s` |
| D | 1k spread (5×200)| `stressbot -prefix spread -n 1000  -enter -c 60 -walk 2s -talk 5s -hold 45s` |
| E | 5k spread (5×1k) | `stressbot -prefix spread -n 5000  -enter -c 60 -walk 2s -talk 5s -hold 45s` |

Key comparisons: **A vs D** (1000 clustered vs spread) and **B vs E** (5000
clustered vs spread). C is the clustered ceiling.

## Measure

While a scenario holds, read the game loop's tick-health (instrumentation from
`l2go-rqc`):
```bash
docker logs --since 60s l2go-gameserver | grep 'game loop tick health'
```
Compare across scenarios: `work_p99`/`work_max` (per-tick budget vs 100 ms),
`behind_ticks` (missed cadence), `max_cmd_depth` (backpressure), and the fleet's
own `entered/failed` + entry latency. Expectation: at equal totals, **spread**
shows markedly lower work spikes and fewer failed entries than **clustered**.

## Notes / caveats

- Until `l2go-cwj` is fixed, clustered 5k/10k will stall on mass entry (multi-second
  ticks) and lose a fraction of entries to timeout — that is the effect under test.
- The client is cheap (~goroutine/bot); the ramp is bounded by the server's
  per-connection RSA keygen, so high counts take minutes to fully enter.
- No server-side max-player gate exists, so >1000 is accepted.
