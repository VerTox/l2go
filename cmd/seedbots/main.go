// Command seedbots pre-creates N stress-test characters directly in the
// GameServer database, reusing the real CharacterUseCase.CreateCharacter path
// (starting items, auto-get skills, spawn location) so each bot is a valid,
// world-entry-ready character.
//
// The l2js-client test harness (references/l2client) can only SELECT an existing
// character by slot — it has no CharacterCreate packet — so characters must be
// prepared out of band. Accounts themselves are auto-created on first login when
// the LoginServer runs with AUTO_CREATE_ACCOUNTS=true, so this tool only touches
// the GameServer database.
//
// Usage:
//
//	go run ./cmd/seedbots -n 100
//	go run ./cmd/seedbots -n 100 -prefix test -dsn postgres://postgres:postgres@127.0.0.1:5432/l2go_gameserver?sslmode=disable
//
// Idempotent: an account that already owns a character is skipped, so re-running
// to top up the pool is safe.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
)

func main() {
	var (
		n         = flag.Int("n", 100, "number of bot characters to ensure exist")
		prefix    = flag.String("prefix", "test", "account name prefix (account = <prefix>NNNN, e.g. test0001)")
		namePfx   = flag.String("name", "Bot", "character name prefix (letters only; index is letter-encoded)")
		dsn       = flag.String("dsn", defaultDSN(), "PostgreSQL DSN for the GameServer database")
		race      = flag.Int("race", int(models.RaceHuman), "race id")
		class     = flag.Int("class", int(models.ClassHumanFighter), "class id")
		sex       = flag.Int("sex", int(models.SexMale), "sex (0=male, 1=female)")
		skillTree = flag.String("skilltree", "datapack/skillTrees/classSkillTree.xml", "path to classSkillTree.xml (for auto-get skills)")
		towns     = flag.String("towns", "", "semicolon-separated x,y,z spawn points to distribute characters across round-robin (empty = single template spawn / clustered)")
	)
	flag.Parse()

	ctx := context.Background()

	townPts, err := parseTowns(*towns)
	if err != nil {
		fatal("bad -towns: %v", err)
	}
	if len(townPts) > 0 {
		fmt.Printf("distributing across %d spawn points (round-robin)\n", len(townPts))
	}

	// Auto-get starting skills need the class skill tree loaded. Best-effort:
	// without it characters still enter the world, just with an empty skill list.
	if err := registry.GetSkillTreeRegistry().LoadFromFile(*skillTree); err != nil {
		fmt.Fprintf(os.Stderr, "warn: could not load skill tree from %q (%v); characters will be created without auto-get skills\n", *skillTree, err)
	} else {
		fmt.Printf("loaded skill tree from %s\n", *skillTree)
	}

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		fatal("connect: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fatal("ping %s: %v", *dsn, err)
	}

	uc := usecase.NewCharacterUseCase(repo.NewPostgreSQLRepository(pool))

	// Character names must be unique server-wide and letters-only. Derive a stem
	// from the account prefix (or -name override) so different scenarios never
	// collide on names. index is letter-encoded onto the stem.
	stem := *namePfx
	if stem == "Bot" {
		stem = nameStem(*prefix)
	}

	var created, skipped, failed int
	start := time.Now()
	for i := 1; i <= *n; i++ {
		account := fmt.Sprintf("%s%04d", *prefix, i)
		name := stem + letterEncode(i)

		// Idempotency: reuse an account's existing character; only create when missing.
		existing, err := uc.GetCharacterList(ctx, account)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [%s] lookup failed: %v\n", account, err)
			failed++
			continue
		}
		var charID int32
		if len(existing) > 0 {
			charID = existing[0].ID
			skipped++
		} else {
			req := &models.CharacterCreateRequest{
				AccountName: account,
				Name:        name,
				Race:        *race,
				Sex:         *sex,
				ClassID:     *class,
			}
			char, err := uc.CreateCharacter(ctx, req)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  [%s] create %q failed: %v\n", account, name, err)
				failed++
				continue
			}
			charID = char.ID
			created++
			if created <= 5 || created%50 == 0 {
				fmt.Printf("  [%s] created %s (char_id=%d)\n", account, char.Name, char.ID)
			}
		}

		// Distribute across towns: override the spawn position round-robin. Applied
		// to both new and existing characters so re-running is idempotent and can
		// reassign coordinates.
		if len(townPts) > 0 {
			t := townPts[(i-1)%len(townPts)]
			if _, err := pool.Exec(ctx, "UPDATE characters SET x=$1, y=$2, z=$3 WHERE char_id=$4", t.x, t.y, t.z, charID); err != nil {
				fmt.Fprintf(os.Stderr, "  [%s] set position failed: %v\n", account, err)
				failed++
			}
		}
	}

	fmt.Printf("\ndone in %s: created=%d skipped=%d failed=%d (target=%d)\n",
		time.Since(start).Round(time.Millisecond), created, skipped, failed, *n)
	if failed > 0 {
		os.Exit(1)
	}
}

// defaultDSN builds a DSN from POSTGRES_* env vars, falling back to the docker
// dev-stack coordinates.
func defaultDSN() string {
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		get("POSTGRES_USERNAME", "postgres"),
		get("POSTGRES_PASSWORD", "postgres"),
		get("POSTGRES_HOST", "127.0.0.1"),
		get("POSTGRES_PORT", "5432"),
		get("POSTGRES_DATABASE", "l2go_gameserver"),
	)
}

// letterEncode turns a positive index into a letters-only, fixed-width string so
// generated names satisfy the server's "letters only, <=16 chars" name rule and
// stay unique. Each decimal digit maps to A..J.
func letterEncode(i int) string {
	s := fmt.Sprintf("%04d", i)
	b := make([]byte, len(s))
	for k := 0; k < len(s); k++ {
		b[k] = 'A' + (s[k] - '0')
	}
	return string(b)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "seedbots: "+format+"\n", args...)
	os.Exit(1)
}

// nameStem builds a letters-only, capitalized character-name stem from an account
// prefix (e.g. "spread" -> "Spread"), capped so stem+index stays within 16 chars.
func nameStem(prefix string) string {
	var b strings.Builder
	for _, r := range prefix {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		s = "Bot"
	}
	if len(s) > 10 {
		s = s[:10]
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

type townPoint struct{ x, y, z int32 }

// parseTowns parses a "x,y,z;x,y,z;..." spec into spawn points. Empty → no points.
func parseTowns(spec string) ([]townPoint, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	var pts []townPoint
	for _, part := range strings.Split(spec, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		f := strings.Split(part, ",")
		if len(f) != 3 {
			return nil, fmt.Errorf("point %q must be x,y,z", part)
		}
		var c [3]int32
		for k := 0; k < 3; k++ {
			v, err := strconv.Atoi(strings.TrimSpace(f[k]))
			if err != nil {
				return nil, fmt.Errorf("point %q: %w", part, err)
			}
			c[k] = int32(v)
		}
		pts = append(pts, townPoint{c[0], c[1], c[2]})
	}
	return pts, nil
}
