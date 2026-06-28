package registry

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// LoadSpawnsFromDirectory loads all fixed-position spawn data from XML files.
// Zone-based spawns (count without coordinates) are skipped in this iteration.
func LoadSpawnsFromDirectory(dir string) ([]models.SpawnData, error) {
	log.Info().Str("dir", dir).Msg("Loading NPC spawns from directory")

	files, err := filepath.Glob(filepath.Join(dir, "*.xml"))
	if err != nil {
		return nil, fmt.Errorf("failed to list spawn XML files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no spawn XML files found in %s", dir)
	}

	var allSpawns []models.SpawnData

	for _, file := range files {
		spawns, err := loadSpawnFile(file)
		if err != nil {
			log.Warn().Err(err).Str("file", file).Msg("Failed to load spawn file")
			continue
		}
		allSpawns = append(allSpawns, spawns...)
	}

	log.Info().Int("total", len(allSpawns)).Int("files", len(files)).Msg("NPC spawns loaded")
	return allSpawns, nil
}

// ---- XML structures for L2J spawn data ----

type xmlSpawnList struct {
	XMLName xml.Name   `xml:"list"`
	Enabled string     `xml:"enabled,attr"`
	Spawns  []xmlSpawn `xml:"spawn"`
}

type xmlSpawn struct {
	Zone string        `xml:"zone,attr"`
	NPCs []xmlSpawnNpc `xml:"npc"`
}

type xmlSpawnNpc struct {
	ID           int32  `xml:"id,attr"`
	X            string `xml:"x,attr"`
	Y            string `xml:"y,attr"`
	Z            string `xml:"z,attr"`
	Heading      string `xml:"heading,attr"`
	RespawnDelay string `xml:"respawnDelay,attr"`
	Count        string `xml:"count,attr"`
}

func loadSpawnFile(filename string) ([]models.SpawnData, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var list xmlSpawnList
	if err := xml.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	// Skip disabled spawn lists
	if list.Enabled == "false" {
		log.Debug().Str("file", filename).Msg("Skipping disabled spawn list")
		return nil, nil
	}

	var spawns []models.SpawnData

	for _, spawnGroup := range list.Spawns {
		for _, npc := range spawnGroup.NPCs {
			// Skip zone-based spawns (have count but no coordinates)
			if npc.X == "" || npc.Y == "" || npc.Z == "" {
				continue
			}

			sd := models.SpawnData{
				NpcID:        npc.ID,
				X:            atoi(npc.X),
				Y:            atoi(npc.Y),
				Z:            atoi(npc.Z),
				Heading:      atoi(npc.Heading),
				RespawnDelay: atoi(npc.RespawnDelay),
				Count:        1,
			}

			if npc.Count != "" {
				sd.Count = atoi(npc.Count)
				if sd.Count < 1 {
					sd.Count = 1
				}
			}

			spawns = append(spawns, sd)
		}
	}

	return spawns, nil
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

// sqlSpawnTupleRe matches a single VALUES tuple from L2J spawnlist.sql:
// ("location", count, npc_templateid, locx, locy, locz, randomx, randomy, heading, respawn_delay, respawn_random, loc_id, periodOfDay)
var sqlSpawnTupleRe = regexp.MustCompile(
	`\("[^"]*"\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*(-?\d+)\s*,\s*(-?\d+)\s*,\s*(-?\d+)\s*,\s*-?\d+\s*,\s*-?\d+\s*,\s*(-?\d+)\s*,\s*(\d+)\s*,\s*-?\d+\s*,\s*-?\d+\s*,\s*\d+\s*\)`,
)

// LoadSpawnsFromSQL loads spawn data by parsing a L2J spawnlist.sql file.
// It extracts INSERT VALUES tuples and converts them to SpawnData entries.
func LoadSpawnsFromSQL(filename string) ([]models.SpawnData, error) {
	log.Info().Str("file", filename).Msg("Loading NPC spawns from SQL file")

	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQL spawn file: %w", err)
	}
	defer f.Close()

	var spawns []models.SpawnData

	scanner := bufio.NewScanner(f)
	// SQL file can have very long lines; increase buffer
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		matches := sqlSpawnTupleRe.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			// m[1]=count, m[2]=npc_templateid, m[3]=locx, m[4]=locy, m[5]=locz, m[6]=heading, m[7]=respawn_delay
			count := atoi(m[1])
			if count < 1 {
				count = 1
			}
			sd := models.SpawnData{
				NpcID:        int32(atoi(m[2])),
				X:            atoi(m[3]),
				Y:            atoi(m[4]),
				Z:            atoi(m[5]),
				Heading:      atoi(m[6]),
				RespawnDelay: atoi(m[7]),
				Count:        count,
			}
			spawns = append(spawns, sd)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading SQL spawn file: %w", err)
	}

	log.Info().Int("total", len(spawns)).Msg("NPC spawns loaded from SQL")
	return spawns, nil
}
