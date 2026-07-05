package gameserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/VerTox/l2go/internal/gameserver/gameloop"
	"github.com/VerTox/l2go/internal/gameserver/handlers/client"
	"github.com/VerTox/l2go/internal/gameserver/handlers/loginserver"
	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
	"github.com/VerTox/l2go/internal/gameserver/schema"
	clienttransport "github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
)

// metricsListenAddr is where the Prometheus /metrics endpoint binds. Fixed to
// match the dev-stack scrape target (docker-compose gameserver:2112). (l2go-5pc)
const metricsListenAddr = ":2112"

type GameServer struct {
	// Configuration
	config config

	// Database
	db   *pgxpool.Pool
	repo repo.DatabaseRepository

	// World state
	world       *registry.WorldRegistry
	connections *registry.ConnectionRegistry

	// skillData is the skill-engine template registry (epic l2go-z36).
	skillData *registry.SkillData

	// Game loop
	gameLoop *gameloop.GameLoop

	// promMetrics exposes game-loop tick-health as Prometheus series over /metrics
	// (l2go-5pc). Created in Run(), served by a worker in run().
	promMetrics *gameloop.PromMetrics

	// Use cases
	usc usecases

	// Handlers
	handlers handlers

	// Connection state
	loginServerHandler *loginserver.Handler

	// Server state
	status gameServerStatus
}

type usecases struct {
	loginServerComm usecase.LoginServerCommUseCase
	playerManager   usecase.PlayerManager
	serverConfig    usecase.ServerConfig
	character       *usecase.CharacterUseCase
	movement        usecase.MovementUseCase
	logout          usecase.LogoutUseCase
	inventory       *usecase.InventoryUseCase
}

type handlers struct {
	loginServer *loginserver.Handler
	// TODO: Add client handlers for game client connections
	client *client.Handler
}

type Params struct {
	// Server configuration
	ServerID   int
	ServerName string
	ServerPort int
	MaxPlayers int

	// LoginServer connection
	LoginServerHost string
	LoginServerPort string

	// Network configuration
	GameHost   string
	GamePort   string
	ExternalIP string

	// Database configuration
	DatabaseURL string

	// Server properties
	ServerType   int
	MinLevel     int
	MaxLevel     int
	AgeLimit     int
	ShowBrackets bool
	PvP          bool
	TestServer   bool
	ShowClock    bool

	// Rates
	ExpRate float64
	SpRate  float64
}

type config struct {
	// Server identity
	serverID   int
	serverName string
	serverPort int
	maxPlayers int

	// LoginServer connection
	loginServerHost string
	loginServerPort string

	// Game server listening
	gameHost   string
	gamePort   string
	externalIP string

	// Database
	databaseURL string

	// Server properties
	serverType   int
	minLevel     int
	maxLevel     int
	ageLimit     int
	showBrackets bool
	pvp          bool
	testServer   bool
	showClock    bool

	// Rates
	expRate float64
	spRate  float64
}

func (c *config) loginServerAddress() string {
	return fmt.Sprintf("%s:%s", c.loginServerHost, c.loginServerPort)
}

func (c *config) gameListener() string {
	return fmt.Sprintf("%s:%s", c.gameHost, c.gamePort)
}

type gameServerStatus struct {
	playersOnline   uint32
	authenticatedLS bool
	connectedToLS   bool
	lastHeartbeat   time.Time

	// Reconnection tracking
	reconnectAttempts int
	lastReconnectTime time.Time
	isReconnecting    bool
}

func (g *GameServer) initDatabase(ctx context.Context) error {
	log.Ctx(ctx).Info().Str("database_url", g.config.databaseURL).Msg("Connecting to PostgreSQL")

	// Create connection pool
	config, err := pgxpool.ParseConfig(g.config.databaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure connection pool
	config.MaxConns = 10
	config.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	g.db = pool
	log.Ctx(ctx).Info().Msg("Connected to PostgreSQL successfully")

	// Run database migrations
	if err := g.runMigrations(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize repository
	g.repo = repo.NewPostgreSQLRepository(pool)
	log.Ctx(ctx).Info().Msg("Database repository initialized")

	// Initialize world registry
	g.world = registry.NewWorldRegistry()
	log.Ctx(ctx).Info().Msg("World registry initialized")

	// Initialize connection registry for player visibility
	g.connections = registry.NewConnectionRegistry()
	log.Ctx(ctx).Info().Msg("Connection registry initialized")

	// Load item templates from XML data files
	// Try multiple paths for flexibility in different environments
	itemTemplatePaths := []string{
		"datapack/stats/items",       // Relative from project root
		"../../datapack/stats/items", // Relative from cmd/gameserver
	}

	templateLoaded := false
	for _, itemTemplateDir := range itemTemplatePaths {
		if err := registry.GetItemTemplateRegistry().LoadFromDirectory(itemTemplateDir); err == nil {
			log.Ctx(ctx).Info().
				Int("count", registry.GetItemTemplateRegistry().Count()).
				Str("dir", itemTemplateDir).
				Msg("Item templates loaded successfully")
			templateLoaded = true
			break
		}
	}

	if !templateLoaded {
		log.Ctx(ctx).Warn().Msg("Failed to load item templates from any path - using fallback values")
	}

	// Load NPC templates from XML data files
	npcTemplatePaths := []string{
		"datapack/stats/npcs",
		"../../datapack/stats/npcs",
	}

	npcTemplatesLoaded := false
	for _, npcDir := range npcTemplatePaths {
		if err := registry.GetNpcTemplateRegistry().LoadFromDirectory(npcDir); err == nil {
			log.Ctx(ctx).Info().
				Int("count", registry.GetNpcTemplateRegistry().Count()).
				Str("dir", npcDir).
				Msg("NPC templates loaded successfully")
			npcTemplatesLoaded = true
			break
		}
	}

	if !npcTemplatesLoaded {
		log.Ctx(ctx).Warn().Msg("Failed to load NPC templates from any path")
	}

	// Load map-region respawn data (nearest-town lookup for revive/teleport). (l2go-3xh.3)
	mapRegionPaths := []string{
		"datapack/mapregion",
		"../../datapack/mapregion",
	}
	for _, dir := range mapRegionPaths {
		if err := registry.GetMapRegionRegistry().LoadFromDirectory(dir); err == nil {
			log.Ctx(ctx).Info().
				Int("count", registry.GetMapRegionRegistry().Count()).
				Str("dir", dir).
				Msg("Map regions loaded successfully")
			break
		}
	}
	if !registry.GetMapRegionRegistry().IsLoaded() {
		log.Ctx(ctx).Warn().Msg("Failed to load map regions from any path")
	}

	// Load class skill trees (auto-get skills granted per class/level). (l2go-3ih)
	for _, path := range []string{
		"datapack/skillTrees/classSkillTree.xml",
		"../../datapack/skillTrees/classSkillTree.xml",
	} {
		if err := registry.GetSkillTreeRegistry().LoadFromFile(path); err == nil {
			log.Ctx(ctx).Info().Str("path", path).Msg("Class skill trees loaded successfully")
			break
		}
	}
	if !registry.GetSkillTreeRegistry().IsLoaded() {
		log.Ctx(ctx).Warn().Msg("Failed to load class skill trees from any path")
	}

	// Load class category data (gates NPC-trainer skill learning by class category). (l2go-hv9)
	for _, path := range []string{
		"datapack/categoryData.xml",
		"../../datapack/categoryData.xml",
	} {
		if err := registry.GetCategoryRegistry().LoadFromFile(path); err == nil {
			log.Ctx(ctx).Info().Str("path", path).Msg("Category data loaded successfully")
			break
		}
	}
	if !registry.GetCategoryRegistry().IsLoaded() {
		log.Ctx(ctx).Warn().Msg("Failed to load category data from any path")
	}

	// Load NPC spawns from database and populate world
	if npcTemplatesLoaded {
		// Seed spawnlist table if empty (one-time import from L2J datapack)
		spawnCount, err := g.repo.Spawn().GetCount(ctx)
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Msg("Failed to check spawnlist count (table may not exist yet)")
		} else if spawnCount == 0 {
			log.Ctx(ctx).Info().Msg("Spawnlist table is empty, seeding from L2J datapack...")
			g.seedSpawnlist(ctx)
		} else {
			log.Ctx(ctx).Info().Int("count", spawnCount).Msg("Spawnlist already populated")
		}

		// Load all spawns from database
		allSpawns, err := g.repo.Spawn().GetAll(ctx)
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Msg("Failed to load spawns from database")
		}

		if len(allSpawns) == 0 {
			log.Ctx(ctx).Warn().Msg("No NPC spawns in database")
		} else {
			npcCount := 0
			skipped := 0
			for _, spawn := range allSpawns {
				tpl := registry.GetNpcTemplateRegistry().Get(spawn.NpcID)
				if tpl == nil {
					skipped++
					continue
				}
				npc := &models.NpcInstance{
					ObjectID:   registry.NextNPCObjectID(),
					TemplateID: spawn.NpcID,
					Template:   tpl,
					Position:   models.Position{X: spawn.X, Y: spawn.Y, Z: spawn.Z},
					Heading:    int32(spawn.Heading),
					IsRunning:  true, // L2J default: NPCs start in running mode (enables idle animation)
					IsDead:     false,
					CurrentHP:  tpl.HP,
					CurrentMP:  tpl.MP,
				}
				g.world.AddNPC(npc)
				npcCount++
			}
			log.Ctx(ctx).Info().
				Int("spawned", npcCount).
				Int("skipped_no_template", skipped).
				Int("total_spawn_entries", len(allSpawns)).
				Msg("NPCs spawned into world")
		}
	}

	return nil
}

// seedSpawnlist imports NPC spawn data from L2J datapack SQL and XML files into the database.
func (g *GameServer) seedSpawnlist(ctx context.Context) {
	var allSpawns []models.SpawnData

	// 1. Primary source: L2J datapack SQL file (~42K entries with town NPCs + field mobs)
	sqlSpawnPaths := []string{
		"datapack/sql/spawnlist.sql",
		"../../datapack/sql/spawnlist.sql",
	}
	for _, sqlPath := range sqlSpawnPaths {
		spawns, err := registry.LoadSpawnsFromSQL(sqlPath)
		if err != nil {
			continue
		}
		allSpawns = append(allSpawns, spawns...)
		log.Ctx(ctx).Info().
			Int("count", len(spawns)).
			Str("file", sqlPath).
			Msg("Parsed SQL spawn data")
		break
	}

	// 2. Additional source: XML spawn files
	xmlSpawnPaths := []string{
		"datapack/spawnlist",
		"../../datapack/spawnlist",
	}
	for _, spawnDir := range xmlSpawnPaths {
		spawns, err := registry.LoadSpawnsFromDirectory(spawnDir)
		if err != nil {
			continue
		}
		allSpawns = append(allSpawns, spawns...)
		log.Ctx(ctx).Info().
			Int("count", len(spawns)).
			Str("dir", spawnDir).
			Msg("Parsed XML spawn data")
		break
	}

	if len(allSpawns) == 0 {
		log.Ctx(ctx).Warn().Msg("No spawn data found to seed")
		return
	}

	// Bulk insert into database
	inserted, err := g.repo.Spawn().BulkInsert(ctx, allSpawns)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Failed to seed spawnlist into database")
		return
	}

	log.Ctx(ctx).Info().
		Int("inserted", inserted).
		Int("parsed", len(allSpawns)).
		Msg("Spawnlist seeded into database")
}

func (g *GameServer) runMigrations(ctx context.Context) error {
	log.Ctx(ctx).Info().Msg("Running database migrations")

	migrator := schema.NewMigrationManager(g.db)
	if err := migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Ctx(ctx).Info().Msg("Database migrations completed successfully")
	return nil
}

func (g *GameServer) prepareUseCases() {
	// Player manager backed by the character repository (real character counts). (l2go-rx4)
	g.usc.playerManager = usecase.NewPlayerManager(g.repo.Character())

	// Initialize server config with real values from service config
	g.usc.serverConfig = usecase.NewServerConfigWithParams(
		g.config.serverID,
		g.config.serverName,
		g.config.maxPlayers,
		g.config.serverPort,
	)

	// Initialize character use case with real repository
	g.usc.character = usecase.NewCharacterUseCase(g.repo)

	// Initialize movement use case
	g.usc.movement = usecase.NewMovementUseCase(g.world, log.Logger)

	// Initialize logout use case
	g.usc.logout = usecase.NewLogoutUseCase(g.world, g.repo.Character(), log.Logger)

	// Initialize inventory use case
	g.usc.inventory = usecase.NewInventoryUseCase(g.repo)

	// Register soulshot / spiritshot item handlers (l2go-sew). The notifier
	// bridges the domain handlers to the world/connection registries for
	// system messages and the activation visual; charged state lives in the
	// global charged-shot registry so combat can later spend it.
	shotNotifier := newShotEffectNotifier(g.world, g.connections)
	charged := registry.GetChargedShotRegistry()
	g.usc.inventory.ItemHandlers().Register("SoulShots", usecase.NewSoulShotHandler(charged, shotNotifier))
	g.usc.inventory.ItemHandlers().Register("SpiritShot", usecase.NewSpiritShotHandler(charged, shotNotifier))
	// BlessedSpiritShot: full weapon shot (separate blessed charge). Beast/Fish
	// shots are PARKED no-ops (l2go-82b): beast needs a pet/summon system, fish
	// needs the fishing system — they never consume until those exist.
	g.usc.inventory.ItemHandlers().Register("BlessedSpiritShot", usecase.NewBlessedSpiritShotHandler(charged, shotNotifier))
	g.usc.inventory.ItemHandlers().Register("BeastSoulShot", usecase.NewBeastShotHandler(shotNotifier))
	g.usc.inventory.ItemHandlers().Register("BeastSpiritShot", usecase.NewBeastShotHandler(shotNotifier))
	g.usc.inventory.ItemHandlers().Register("FishShots", usecase.NewFishShotHandler())

	// Initialize LoginServer communication use case with callbacks
	g.usc.loginServerComm = usecase.NewLoginServerCommUseCaseWithCallbacks(
		g.usc.playerManager,
		g.usc.serverConfig,
		g.SetAuthenticated, // Callback to update authentication status
		g.SendAuthRequest,  // Callback to send AuthRequest after InitLS
		// Callback to reply with the account's character count. loginServerHandler is set
		// in prepareHandlers (right after this), well before any RequestCharacters arrives.
		func(account string, charCount, charsInDel int) error {
			return g.loginServerHandler.SendReplyCharacters(account, charCount, charsInDel)
		},
	)
}

func (g *GameServer) prepareHandlers() {
	// Initialize LoginServer handler
	g.handlers.loginServer = loginserver.New(g.usc.loginServerComm)
	g.loginServerHandler = g.handlers.loginServer

	// Set disconnection callback for LoginServer
	g.handlers.loginServer.SetDisconnectionCallback(g)

	// Initialize client handlers for game client connections with use cases
	g.handlers.client = client.New(g.usc.character, g.usc.movement, g.usc.logout, g.usc.inventory, g.world, g.connections, g.handlers.loginServer, g.gameLoop.CommandChannel())

	// Register consumable item handlers. INTERIM (l2go-diu): potions read their
	// linked restore skill (HP/MP/CP + amount) from the skill data and restore
	// immediately via the game loop, until a real skill engine replaces this.
	skillRoots := []string{
		"datapack/stats/skills",
		"../../datapack/stats/skills",
	}
	// Skill engine template registry (epic l2go-z36). Lazily parses the skill
	// datapack into per-(id,level) templates. Wired into the client handler so the
	// SkillList packet can flag passive skills correctly.
	g.skillData = registry.NewSkillData(skillRoots)
	g.handlers.client.SetSkillData(g.skillData)
	g.handlers.client.SetPromMetrics(g.promMetrics) // world-entry funnel (l2go-5wq)
	g.gameLoop.SetSkillData(g.skillData) // casting (l2go-lu8)

	// Potions cast their linked item skill through the real skill engine (l2go-849):
	// validate the template via SkillData, cast via the loop's ItemSkillCaster.
	itemSkillHandler := usecase.NewItemSkillHandler(g.skillData, g.gameLoop.ItemSkillCaster())
	g.usc.inventory.ItemHandlers().Register("ItemSkills", itemSkillHandler)
	g.usc.inventory.ItemHandlers().Register("ManaPotion", itemSkillHandler)

	// Register extractable (lootbox/capsule) item handler (l2go-7j7). Rewards are
	// rolled per-product and added to inventory; rewards ride the used item's
	// InventoryUpdate via ItemUseContext.Emit.
	g.usc.inventory.ItemHandlers().Register("ExtractableItems", usecase.NewExtractableItemsHandler())

	// Register the recipe-scroll item handler (l2go-9sw). Using a recipe scroll
	// registers the recipe in the character's recipe book (character_recipes) and
	// consumes one scroll. Recipes are resolved from recipes.xml by the scroll's
	// item id; the notifier delivers SystemMessage feedback (reusing the shot
	// notifier, which now also satisfies usecase.RecipeNotifier).
	recipes := registry.NewRecipeRegistry([]string{
		"datapack",
		"../../datapack",
	})
	recipeNotifier := newShotEffectNotifier(g.world, g.connections)
	g.usc.inventory.ItemHandlers().Register("Recipes", usecase.NewRecipeHandler(recipes, recipeNotifier))

	// Enchant scrolls (l2go-f16): two-step flow. The "EnchantScrolls" item handler
	// arms a scroll and prompts the client (ChooseInventoryItem); RequestEnchantItem
	// then performs the enchant. Tuning (target grade/caps/bonus) loads from
	// enchantItemData.xml; success chance is data-driven from enchantItemGroups.xml.
	enchantData := registry.NewEnchantDataRegistry()
	if err := enchantData.LoadFromFile(
		"datapack/enchantItemData.xml",
		"../../datapack/enchantItemData.xml",
	); err != nil {
		log.Warn().Err(err).Msg("failed to load enchant item data; enchant scrolls disabled")
	}
	enchantGroups := registry.NewEnchantGroupsRegistry()
	if err := enchantGroups.LoadFromFile(
		"datapack/enchantItemGroups.xml",
		"../../datapack/enchantItemGroups.xml",
	); err != nil {
		log.Warn().Err(err).Msg("failed to load enchant item groups; enchant success chance unavailable")
	}
	enchantNotifier := newEnchantNotifier(g.world, g.connections)
	enchantUC := usecase.NewEnchantUseCase(g.repo, enchantData, enchantGroups, registry.GetEnchantStateRegistry(), enchantNotifier, nil)
	g.usc.inventory.ItemHandlers().Register("EnchantScrolls", enchantUC.ScrollHandler())
	g.handlers.client.SetEnchantUseCase(enchantUC)
}

// connectToLoginServerWithRetry connects to LoginServer with retry logic
func (g *GameServer) connectToLoginServerWithRetry(ctx context.Context) error {
	const maxRetries = 5 // Try 5 times on startup, then switch to background reconnect
	const retryDelay = 5 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Ctx(ctx).Info().
			Int("attempt", attempt).
			Int("max_attempts", maxRetries).
			Str("address", g.config.loginServerAddress()).
			Msg("Attempting to connect to LoginServer")

		err := g.connectToLoginServer(ctx)
		if err == nil {
			g.status.reconnectAttempts = 0
			g.status.isReconnecting = false
			return nil
		}

		log.Ctx(ctx).Warn().
			Err(err).
			Int("attempt", attempt).
			Msg("LoginServer connection failed, retrying...")

		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
				// Continue to next attempt
			}
		}
	}

	// Start background reconnection after initial attempts fail
	log.Ctx(ctx).Warn().
		Int("failed_attempts", maxRetries).
		Msg("Initial LoginServer connection failed, starting background reconnection")

	g.startBackgroundReconnection(ctx)
	return nil // Don't fail GameServer startup
}

func (g *GameServer) connectToLoginServer(ctx context.Context) error {
	log.Ctx(ctx).Info().
		Str("address", g.config.loginServerAddress()).
		Msg("Connecting to LoginServer")

	err := g.loginServerHandler.ConnectToLoginServer(ctx, g.config.loginServerAddress())
	if err != nil {
		return fmt.Errorf("failed to connect to LoginServer: %w", err)
	}

	g.status.connectedToLS = true
	log.Ctx(ctx).Info().Msg("Connected to LoginServer successfully - waiting for InitLS")

	// DO NOT send AuthRequest here - wait for InitLS packet first!
	// AuthRequest will be sent after receiving InitLS and sending BlowFishKey
	return nil
}

// startBackgroundReconnection starts background LoginServer reconnection
func (g *GameServer) startBackgroundReconnection(ctx context.Context) {
	const reconnectInterval = 30 * time.Second // Fixed 30 second interval

	g.status.isReconnecting = true

	go func() {
		ticker := time.NewTicker(reconnectInterval)
		defer ticker.Stop()

		log.Ctx(ctx).Info().
			Dur("interval", reconnectInterval).
			Msg("Started background LoginServer reconnection")

		for {
			select {
			case <-ctx.Done():
				log.Ctx(ctx).Info().Msg("Stopping background LoginServer reconnection due to context cancellation")
				return
			case <-ticker.C:
				if g.status.connectedToLS {
					// Already connected, stop background reconnection
					log.Ctx(ctx).Info().Msg("LoginServer connection restored, stopping background reconnection")
					g.status.isReconnecting = false
					return
				}

				g.status.reconnectAttempts++
				g.status.lastReconnectTime = time.Now()

				log.Ctx(ctx).Info().
					Int("attempt", g.status.reconnectAttempts).
					Msg("Attempting background LoginServer reconnection")

				if err := g.connectToLoginServer(ctx); err != nil {
					log.Ctx(ctx).Warn().
						Err(err).
						Int("attempt", g.status.reconnectAttempts).
						Dur("next_retry", reconnectInterval).
						Msg("Background LoginServer reconnection failed")
				} else {
					log.Ctx(ctx).Info().
						Int("total_attempts", g.status.reconnectAttempts).
						Msg("Background LoginServer reconnection successful")
					g.status.reconnectAttempts = 0
					g.status.isReconnecting = false
					return
				}
			}
		}
	}()
}

// OnLoginServerDisconnected handles LoginServer disconnection
func (g *GameServer) OnLoginServerDisconnected() {
	log.Info().Msg("LoginServer connection lost")

	g.status.connectedToLS = false
	g.status.authenticatedLS = false

	// Start background reconnection if not already running
	if !g.status.isReconnecting {
		ctx := context.Background() // Use background context for reconnection
		log.Info().Msg("Starting background reconnection after disconnection")
		g.startBackgroundReconnection(ctx)
	}
}

// SendAuthRequest sends AuthRequest to LoginServer (called after InitLS processing)
func (g *GameServer) SendAuthRequest(ctx context.Context) error {
	return g.sendAuthRequest(ctx)
}

// buildServerAddresses builds the paired subnet/host lists reported to the
// LoginServer. The LoginServer relays the host matching a connecting client's
// IP back to that client. Local clients (127.0.0.0/8) always get 127.0.0.1;
// everyone else (0.0.0.0/0 fallback) gets the configured external host.
//
// externalHost may be an IP literal or a hostname. It is passed through as-is
// so the LoginServer can resolve hostnames; only a valid IP literal is
// normalized via net.IP.String(). An empty value falls back to 127.0.0.1 to
// avoid advertising an unreachable "<nil>" address.
func buildServerAddresses(externalHost string) (subnets, hosts []string) {
	external := externalHost
	if external == "" {
		external = "127.0.0.1"
	} else if ip := net.ParseIP(external); ip != nil {
		external = ip.String()
	}

	subnets = []string{"127.0.0.0/8", "0.0.0.0/0"}
	hosts = []string{"127.0.0.1", external}
	return subnets, hosts
}

func (g *GameServer) sendAuthRequest(ctx context.Context) error {
	log.Ctx(ctx).Info().Msg("Sending AuthRequest to LoginServer")

	// TODO: Generate proper hex ID for server authentication
	// For now, use a simple placeholder
	hexID := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	subnets, hosts := buildServerAddresses(g.config.externalIP)

	err := g.loginServerHandler.SendAuthRequest(g.usc.serverConfig, hexID, subnets, hosts)
	if err != nil {
		return fmt.Errorf("failed to send AuthRequest: %w", err)
	}

	log.Ctx(ctx).Info().Msg("AuthRequest sent to LoginServer")
	return nil
}

func (g *GameServer) startHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Send heartbeat every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if g.status.connectedToLS && g.status.authenticatedLS {
				g.sendServerStatus(ctx)
			}
		}
	}
}

func (g *GameServer) startWorldCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Cleanup offline players (remove after 10 minutes offline)
			removed := g.world.CleanupOfflinePlayers(ctx, 10*time.Minute)
			if removed > 0 {
				log.Ctx(ctx).Info().Int("removed", removed).Msg("World cleanup completed")
			}
		}
	}
}

func (g *GameServer) sendServerStatus(ctx context.Context) {
	log.Ctx(ctx).Debug().Msg("Sending ServerStatus heartbeat to LoginServer")

	err := g.loginServerHandler.SendServerStatus(
		g.config.serverID,
		1, // Online status
		g.config.serverPort,
		g.config.maxPlayers,
		g.config.serverType,
		g.config.minLevel,
		g.config.maxLevel,
		g.config.ageLimit,
		g.config.showBrackets,
		g.config.pvp,
		g.config.testServer,
		g.config.showClock,
	)

	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Failed to send ServerStatus")
	} else {
		g.status.lastHeartbeat = time.Now()
		log.Ctx(ctx).Debug().Msg("ServerStatus sent successfully")
	}
}

func New(p Params) *GameServer {
	return &GameServer{
		config: config{
			serverID:        p.ServerID,
			serverName:      p.ServerName,
			serverPort:      p.ServerPort,
			maxPlayers:      p.MaxPlayers,
			loginServerHost: p.LoginServerHost,
			loginServerPort: p.LoginServerPort,
			gameHost:        p.GameHost,
			gamePort:        p.GamePort,
			externalIP:      p.ExternalIP,
			databaseURL:     p.DatabaseURL,
			serverType:      p.ServerType,
			minLevel:        p.MinLevel,
			maxLevel:        p.MaxLevel,
			ageLimit:        p.AgeLimit,
			showBrackets:    p.ShowBrackets,
			pvp:             p.PvP,
			testServer:      p.TestServer,
			showClock:       p.ShowClock,
		},
		status: gameServerStatus{
			playersOnline:   0,
			authenticatedLS: false,
			connectedToLS:   false,
		},
	}
}

func (g *GameServer) Run(ctx context.Context) error {
	log.Ctx(ctx).Info().
		Int("server_id", g.config.serverID).
		Str("server_name", g.config.serverName).
		Msg("Starting GameServer")

	// Initialize database first
	if err := g.initDatabase(ctx); err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}

	// Create game loop before use cases (handlers need the command channel)
	g.gameLoop = gameloop.New(g.world, g.connections, g.config.expRate, g.config.spRate)

	// Wire Prometheus tick-health collectors; the loop feeds them each tick and the
	// /metrics worker in run() exposes them for scraping. (l2go-5pc)
	g.promMetrics = gameloop.NewPromMetrics()
	g.gameLoop.SetPromMetrics(g.promMetrics)

	// Seed respawn data from the NPCs loaded into the world above, so killed NPCs
	// can respawn (otherwise RespawnEvent finds no spawn info). (l2go-c44)
	g.gameLoop.RegisterWorldSpawns()

	g.prepareUseCases()
	g.prepareHandlers()

	// Connect to LoginServer after database is ready (with retry logic)
	if err := g.connectToLoginServerWithRetry(ctx); err != nil {
		return fmt.Errorf("LoginServer connection setup failed: %w", err)
	}

	return g.run(ctx)
}

func (g *GameServer) run(ctx context.Context) error {
	// Async character saver: the game loop enqueues value-copy snapshots (autosave +
	// level-up) here, and this goroutine performs the DB writes so persistence latency
	// never stalls the loop. It lives outside the errgroup so it stays alive through
	// shutdown to flush anything still queued before the DB closes.
	saveCh := make(chan models.Character, 256)
	saverDone := make(chan struct{})
	go func() {
		defer close(saverDone)
		for char := range saveCh {
			if err := g.repo.Character().Update(context.Background(), &char); err != nil {
				log.Ctx(ctx).Error().Err(err).Int32("char_id", char.ID).Msg("autosave: failed to persist character")
			}
		}
	}()
	g.gameLoop.SetPersistSink(saveCh)

	// Async auto-soulshot recharge: the game loop enqueues charIDs whose active
	// auto-shots must be recharged to arm the next swing. The DB consume + charge
	// run here so the tick never blocks on the database. When a shot runs out it is
	// auto-disabled; echo ExAutoSoulShot(off) + the cancel message to un-highlight
	// the client icon. Closed after the loop stops (below) so no send races the close.
	rechargeCh := make(chan int32, 256)
	rechargeDone := make(chan struct{})
	go func() {
		defer close(rechargeDone)
		for charID := range rechargeCh {
			consumed, disabled, err := g.usc.inventory.RechargeAutoShots(context.Background(), charID)
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Int32("char_id", charID).Msg("auto-shot recharge failed")
				continue
			}
			// Refresh the shot count in the client bag after auto-consume.
			g.handlers.client.SendInventoryUpdate(charID, consumed)
			if len(disabled) == 0 {
				continue
			}
			player, ok := g.world.GetPlayer(charID)
			if !ok {
				continue
			}
			conn := g.connections.GetConnection(player.AccountName)
			if conn == nil {
				continue
			}
			for _, itemID := range disabled {
				_ = conn.Send(outclient.BuildExAutoSoulShot(itemID, 0))
				_ = conn.Send(outclient.NewSystemMessage(outclient.SysMsgAutoUseOfS1Cancelled).AddItemName(itemID).Build())
			}
		}
	}()
	g.gameLoop.SetAutoShotSink(rechargeCh)

	// Async skill-learn persistence: the game loop deducts SP + updates the live
	// known-skills map on its goroutine, then enqueues the learned skill here so the
	// DB write never blocks the tick. (l2go-hv9)
	learnCh := make(chan gameloop.LearnedSkill, 256)
	learnDone := make(chan struct{})
	go func() {
		defer close(learnDone)
		for ls := range learnCh {
			if err := g.repo.Skill().LearnSkill(context.Background(), ls.CharID, ls.SkillID, int(ls.Level)); err != nil {
				log.Ctx(ctx).Error().Err(err).Int32("char_id", ls.CharID).Int32("skill", ls.SkillID).Msg("failed to persist learned skill")
			}
		}
	}()
	g.gameLoop.SetSkillLearnSink(learnCh)

	// Expose the async persistence sinks' backlog as Prometheus gauges (l2go-f9j).
	// Read via len() at scrape time — no sampler goroutine. A filling queue means DB
	// latency is outpacing the loop and about to stall the tick; the earliest scalable
	// warning sign under load.
	g.promMetrics.RegisterQueueDepth("l2go_sink_save_queue_depth", "Pending character-persistence snapshots queued for the async saver.", func() int { return len(saveCh) })
	g.promMetrics.RegisterQueueDepth("l2go_sink_recharge_queue_depth", "Pending auto-soulshot recharge requests queued off the loop.", func() int { return len(rechargeCh) })
	g.promMetrics.RegisterQueueDepth("l2go_sink_learn_queue_depth", "Pending learned-skill writes queued for async persistence.", func() int { return len(learnCh) })
	// Active client connections gauge (l2go-18n) — live count read at scrape time.
	g.promMetrics.RegisterQueueDepth("l2go_active_connections", "Registered client TCP connections.", func() int { return g.connections.GetConnectionCount() })

	eg, egctx := errgroup.WithContext(ctx)

	// Start heartbeat routine
	eg.Go(func() error {
		g.startHeartbeat(egctx)
		return nil
	})

	// Start world cleanup routine
	eg.Go(func() error {
		g.startWorldCleanup(egctx)
		return nil
	})

	// Start game loop
	eg.Go(func() error {
		return g.gameLoop.Run(egctx)
	})

	// Serve Prometheus tick-health metrics on /metrics. Bound to a fixed port that
	// the dev-stack Prometheus scrapes (l2go-5pc); shuts down gracefully when the
	// group context is cancelled so it never outlives the rest of the server.
	eg.Go(func() error {
		mux := http.NewServeMux()
		mux.Handle("/metrics", g.promMetrics.Handler())
		srv := &http.Server{Addr: metricsListenAddr, Handler: mux}
		go func() {
			<-egctx.Done()
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutCtx)
		}()
		log.Ctx(ctx).Info().Str("addr", metricsListenAddr).Msg("Metrics endpoint listening on /metrics")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("metrics server: %w", err)
		}
		return nil
	})

	// Start game client listener
	eg.Go(func() error {
		return clienttransport.ListenAndServe(egctx, g.config.gameListener(), g.handlers.client.Handle)
	})

	// Wait for every worker — the game loop included — to stop before touching shared
	// state. Once this returns, no goroutine mutates character progress anymore, so
	// the shutdown save below is race-free.
	err := eg.Wait()

	log.Ctx(ctx).Info().Msg("GameServer shutting down...")

	// Flush queued autosave snapshots first (they are older), then close the saver so
	// the authoritative shutdown save below wins over any stale queued copy.
	close(saveCh)
	<-saverDone

	// The loop has stopped, so no more recharge requests can be enqueued — safe to
	// close the recharge sink and let its goroutine drain and exit.
	close(rechargeCh)
	<-rechargeDone

	// Same for the skill-learn sink: the loop has stopped, drain and exit.
	close(learnCh)
	<-learnDone

	// Save-on-shutdown: persist the freshest snapshot of every online player before
	// the DB closes, so a graceful stop never loses session progress.
	g.saveOnlinePlayersOnShutdown(context.Background())

	// Disconnect from LoginServer
	if g.loginServerHandler != nil {
		g.loginServerHandler.Disconnect()
	}

	// Close database connection (after all saves have completed)
	if g.db != nil {
		g.db.Close()
		log.Ctx(ctx).Info().Msg("Database connection closed")
	}

	log.Ctx(ctx).Info().Msg("GameServer shutdown complete")
	return err
}

// saveOnlinePlayersOnShutdown persists every online player's character to the DB.
// Called after the game loop has stopped, so character progress fields are stable;
// the snapshot is taken under the registry lock to serialize with any lingering
// connection goroutines still writing position.
func (g *GameServer) saveOnlinePlayersOnShutdown(ctx context.Context) {
	snapshots := g.world.SnapshotOnlineCharacters()
	if len(snapshots) == 0 {
		return
	}

	saved := 0
	for i := range snapshots {
		if err := g.repo.Character().Update(ctx, &snapshots[i]); err != nil {
			log.Ctx(ctx).Error().Err(err).Int32("char_id", snapshots[i].ID).Msg("save-on-shutdown: failed to persist character")
			continue
		}
		saved++
	}

	log.Ctx(ctx).Info().Int("saved", saved).Int("total", len(snapshots)).Msg("Saved online players on shutdown")
}

// GetStatus returns current server status
func (g *GameServer) GetStatus() gameServerStatus {
	return g.status
}

// SetAuthenticated marks the server as authenticated with LoginServer
func (g *GameServer) SetAuthenticated(authenticated bool) {
	g.status.authenticatedLS = authenticated
	if authenticated {
		log.Info().Msg("GameServer authenticated with LoginServer")
	}
}

// GetPlayersOnline returns the current number of players online
func (g *GameServer) GetPlayersOnline() uint32 {
	return g.status.playersOnline
}

// SetPlayersOnline updates the current number of players online
func (g *GameServer) SetPlayersOnline(count uint32) {
	g.status.playersOnline = count
}
