// Command stressbot is the native Go stress-test client (epic l2go-mjg): one
// goroutine per bot instead of the Node harness's one-process-per-bot, so a
// fleet of 1000+ bots fits in a few MB. It reuses the server's own crypto
// (pkg/crypt, gamecrypt) and packet layouts, guaranteeing byte-compatibility.
//
// Modes:
//
//	stressbot -user stress0001                       # one login, print keys
//	stressbot -user stress0001 -enter                # one bot, enter world, stay online
//	stressbot -n 200 -c 50                            # 200 concurrent logins (throughput)
//	stressbot -n 1000 -c 50 -enter -hold 60s         # 1000 bots online for 60s (load test)
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/VerTox/l2go/internal/stressclient"
)

func main() {
	var (
		addr    = flag.String("addr", "127.0.0.1:2106", "LoginServer address")
		gsAddr  = flag.String("gs", "127.0.0.1:7777", "GameServer address (ServerList advertises an internal IP, so override here)")
		user    = flag.String("user", "stress0001", "single account name (when -n not set)")
		pass    = flag.String("pass", "", "password (default: same as account name)")
		enter   = flag.Bool("enter", false, "run the GameServer handshake and enter the world")
		slot    = flag.Int("slot", 0, "character slot to select on enter")
		n       = flag.Int("n", 0, "fleet size (0 = single -user run)")
		prefix  = flag.String("prefix", "stress", "account prefix for fleet mode (account = <prefix>NNNN)")
		start   = flag.Int("start", 1, "first account index for fleet mode")
		conc    = flag.Int("c", 50, "max concurrent entries during ramp")
		hold    = flag.Duration("hold", 0, "with -enter: hold the fleet online this long after ramp (0 = until Ctrl+C)")
		walk     = flag.Duration("walk", 0, "with -enter: each bot random-walks every N (0 = stand still)")
		talk     = flag.Duration("talk", 0, "with -enter: each bot says a random chat line every N (0 = silent)")
		radius   = flag.Int("radius", 200, "random-walk radius in game units")
		timeout  = flag.Duration("timeout", 15*time.Second, "per-step timeout")
		prom     = flag.String("prom", "http://localhost:9090", "Prometheus base URL for the tick-health summary after a fleet run (empty = off)")
		promsnap = flag.Bool("promsnap", false, "just print the tick-health summary over -window and exit (no fleet run)")
		window   = flag.Duration("window", 3*time.Minute, "look-back window for -promsnap")
	)
	flag.Parse()

	if *promsnap {
		printPromSummary(*prom, time.Now().Add(-*window), time.Now())
		return
	}

	cfg := runCfg{ls: *addr, gs: *gsAddr, enter: *enter, slot: *slot, timeout: *timeout,
		walk: *walk, talk: *talk, radius: *radius, prom: *prom}
	switch {
	case *n <= 0:
		runSingle(cfg, *user, pw(*pass, *user))
	case cfg.enter:
		runFleetWorld(cfg, *prefix, *start, *n, *conc, *hold)
	default:
		runFleetLogin(cfg, *prefix, *start, *n, *conc)
	}
}

// runCfg carries the shared connection settings for a run.
type runCfg struct {
	ls, gs     string
	enter      bool
	slot       int
	timeout    time.Duration
	walk, talk time.Duration
	radius     int
	prom       string
}

// chatLines are short, generic messages bots pick from when -talk is on.
var chatLines = []string{
	"hi", "hello there", "anyone here", "wtb adena", "wts drops cheap",
	"lf party", "gg", "buffs pls", "grinding time", "need help pls",
	"lag", "back", "brb", "gl hf", "where is the shop",
}

func loginAndMaybeEnter(cfg runCfg, account, password string) (*stressclient.GameSession, error) {
	keys, err := stressclient.Login(cfg.ls, account, password, cfg.timeout)
	if err != nil {
		return nil, err
	}
	if !cfg.enter {
		return nil, nil
	}
	return stressclient.EnterWorld(cfg.gs, account, cfg.slot, keys, cfg.timeout)
}

func pw(pass, user string) string {
	if pass == "" {
		return user
	}
	return pass
}

func acct(prefix string, idx int) string { return fmt.Sprintf("%s%04d", prefix, idx) }

func runSingle(cfg runCfg, user, password string) {
	t := time.Now()
	sess, err := loginAndMaybeEnter(cfg, user, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", user, err)
		os.Exit(1)
	}
	if !cfg.enter {
		fmt.Printf("OK %s logged in via %s in %s\n", user, cfg.ls, time.Since(t).Round(time.Millisecond))
		return
	}
	fmt.Printf("OK %s entered world (slot %d) in %s\n", user, cfg.slot, time.Since(t).Round(time.Millisecond))
	defer sess.Close()
	for {
		if err := sess.Drain(); err != nil {
			fmt.Fprintf(os.Stderr, "%s disconnected: %v\n", user, err)
			return
		}
	}
}

// runFleetLogin runs n LoginServer flows concurrently and reports throughput and
// latency — the login-only stress path (no world entry).
func runFleetLogin(cfg runCfg, prefix string, start, n, conc int) {
	fmt.Printf("fleet: %d logins, concurrency %d, ls=%s\n", n, conc, cfg.ls)
	var ok, fail int64
	var mu sync.Mutex
	latencies := make([]time.Duration, 0, n)
	errCounts := map[string]int{}

	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	begin := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			a := acct(prefix, start+idx)
			t := time.Now()
			_, err := stressclient.Login(cfg.ls, a, a, cfg.timeout)
			lat := time.Since(t)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fail++
				errCounts[err.Error()]++
				return
			}
			ok++
			latencies = append(latencies, lat)
		}(i)
	}
	wg.Wait()
	total := time.Since(begin)
	fmt.Printf("\ndone in %s: ok=%d fail=%d  throughput=%.0f/s\n",
		total.Round(time.Millisecond), ok, fail, float64(ok)/total.Seconds())
	printLatency(latencies)
	printErrors(errCounts)
}

// runFleetWorld ramps n bots into the world (bounded by conc concurrent entries),
// then holds them all online — each goroutine drains its socket as keepalive —
// while a monitor prints the live online count and client RSS. This is the load
// generator for measuring the server's game-loop tick health under N players.
func runFleetWorld(cfg runCfg, prefix string, start, n, conc int, hold time.Duration) {
	fmt.Printf("fleet: entering %d bots (entry concurrency %d), ls=%s gs=%s\n", n, conc, cfg.ls, cfg.gs)
	var entered, failed, online int64
	var mu sync.Mutex
	latencies := make([]time.Duration, 0, n)
	errCounts := map[string]int{}
	sessions := make([]*stressclient.GameSession, 0, n)

	sem := make(chan struct{}, conc)
	var entryWG sync.WaitGroup
	begin := time.Now()

	// Live monitor.
	stopMon := make(chan struct{})
	go func() {
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-stopMon:
				return
			case <-tick.C:
				fmt.Printf("[%4.0fs] online=%d entered=%d failed=%d  rss=%s\n",
					time.Since(begin).Seconds(),
					atomic.LoadInt64(&online), atomic.LoadInt64(&entered),
					atomic.LoadInt64(&failed), rss())
			}
		}
	}()

	for i := 0; i < n; i++ {
		entryWG.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			a := acct(prefix, start+idx)
			t := time.Now()
			sess, err := loginAndMaybeEnter(cfg, a, a)
			lat := time.Since(t)
			<-sem // release the entry slot as soon as we're in (ramp continues)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				mu.Lock()
				errCounts[err.Error()]++
				mu.Unlock()
				entryWG.Done()
				return
			}
			atomic.AddInt64(&entered, 1)
			atomic.AddInt64(&online, 1)
			mu.Lock()
			latencies = append(latencies, lat)
			sessions = append(sessions, sess)
			mu.Unlock()
			entryWG.Done()

			// Behaviour: a separate goroutine owns all sends (walk/chat) so it never
			// races the drain-reader below. Sends stop when the connection closes.
			if cfg.walk > 0 || cfg.talk > 0 {
				go botBehaviour(sess, idx, cfg)
			}

			// Stay online: drain the socket until disconnected/torn down.
			for {
				if err := sess.Drain(); err != nil {
					atomic.AddInt64(&online, -1)
					return
				}
			}
		}(i)
	}

	entryWG.Wait()
	rampDur := time.Since(begin)
	fmt.Printf("\nramp done in %s: entered=%d failed=%d (online=%d)\n",
		rampDur.Round(time.Millisecond), atomic.LoadInt64(&entered), atomic.LoadInt64(&failed), atomic.LoadInt64(&online))
	printLatency(latencies)
	printErrors(errCounts)
	fmt.Printf("client RSS: %s\n", rss())

	// Hold the fleet online.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	if hold > 0 {
		fmt.Printf("holding %d bots online for %s (Ctrl+C to stop early) ...\n", atomic.LoadInt64(&online), hold)
		select {
		case <-time.After(hold):
		case <-sig:
		}
	} else {
		fmt.Printf("holding %d bots online — Ctrl+C to tear down ...\n", atomic.LoadInt64(&online))
		<-sig
	}

	close(stopMon)
	fmt.Println("tearing down ...")
	mu.Lock()
	for _, s := range sessions {
		s.Close()
	}
	mu.Unlock()
	time.Sleep(500 * time.Millisecond)
	fmt.Printf("final online=%d\n", atomic.LoadInt64(&online))

	// One combined report: pull the loop's tick health over the whole run from
	// Prometheus, right next to the fleet stats above.
	if cfg.prom != "" {
		printPromSummary(cfg.prom, begin, time.Now())
	}
}

func printLatency(latencies []time.Duration) {
	if len(latencies) == 0 {
		return
	}
	slices.Sort(latencies)
	p := func(q float64) time.Duration { return latencies[int(q*float64(len(latencies)-1))] }
	fmt.Printf("entry latency: p50=%s p95=%s p99=%s max=%s\n",
		p(0.50).Round(time.Millisecond), p(0.95).Round(time.Millisecond),
		p(0.99).Round(time.Millisecond), latencies[len(latencies)-1].Round(time.Millisecond))
}

func printErrors(errCounts map[string]int) {
	if len(errCounts) == 0 {
		return
	}
	fmt.Println("errors:")
	for e, c := range errCounts {
		fmt.Printf("  %d× %s\n", c, e)
	}
}

// botBehaviour drives one bot's walk and chat on independent timers. It owns all
// sends for that session, so it never races the drain-reader. Returns when a send
// fails (connection closed at teardown). idx seeds a per-bot RNG so paths differ.
func botBehaviour(sess *stressclient.GameSession, idx int, cfg runCfg) {
	rng := rand.New(rand.NewSource(int64(idx) + 1))
	sx, sy, sz := sess.Spawn()

	var walkC, talkC <-chan time.Time
	if cfg.walk > 0 {
		t := time.NewTicker(cfg.walk)
		defer t.Stop()
		walkC = t.C
	}
	if cfg.talk > 0 {
		t := time.NewTicker(cfg.talk)
		defer t.Stop()
		talkC = t.C
	}
	for {
		select {
		case <-walkC:
			dx := int32(rng.Intn(2*cfg.radius+1) - cfg.radius)
			dy := int32(rng.Intn(2*cfg.radius+1) - cfg.radius)
			if err := sess.Walk(sx+dx, sy+dy, sz); err != nil {
				return
			}
		case <-talkC:
			if err := sess.Say(chatLines[rng.Intn(len(chatLines))]); err != nil {
				return
			}
		}
	}
}

// rss reports the Go runtime's memory footprint (heap in use / total from OS).
func rss() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return fmt.Sprintf("heap=%dMB sys=%dMB", m.HeapAlloc>>20, m.Sys>>20)
}
