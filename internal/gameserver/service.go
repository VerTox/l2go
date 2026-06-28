package gameserver

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/VerTox/l2go/internal/gameserver/gameloop"
	"github.com/VerTox/l2go/internal/gameserver/handlers/client"
	"github.com/VerTox/l2go/internal/gameserver/handlers/loginserver"
	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
	"github.com/VerTox/l2go/internal/gameserver/schema"
	clienttransport "github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
)

type GameServer struct {
	// Configuration
	config config

	// Database
	db   *pgxpool.Pool
	repo repo.DatabaseRepository

	// World state
	world       *registry.WorldRegistry
	connections *registry.ConnectionRegistry

	// Game loop
	gameLoop *gameloop.GameLoop

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
		"data/stats/items",           // Relative from project root
		"../../data/stats/items",     // Relative from cmd/gameserver
		"./data/stats/items",         // Explicit current directory
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
		"data/stats/npcs",
		"../../data/stats/npcs",
		"references/data/stats/npcs",
		"../../references/data/stats/npcs",
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
		"references/l2jserver-l2j-server-datapack-f39d964439a9/src/main/resources/sql/spawnlist.sql",
		"../../references/l2jserver-l2j-server-datapack-f39d964439a9/src/main/resources/sql/spawnlist.sql",
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
		"data/spawnlist",
		"../../data/spawnlist",
		"references/data/spawnlist",
		"../../references/data/spawnlist",
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
	// Initialize mock implementations (TODO: replace with real implementations)
	g.usc.playerManager = usecase.NewMockPlayerManager()

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
	g.usc.movement = usecase.NewMovementUseCase(g.world, g.repo.Character(), log.Logger)
	
	// Initialize logout use case
	g.usc.logout = usecase.NewLogoutUseCase(g.world, g.repo.Character(), log.Logger)

	// Initialize inventory use case
	g.usc.inventory = usecase.NewInventoryUseCase(g.repo)

	// Initialize LoginServer communication use case with callbacks
	g.usc.loginServerComm = usecase.NewLoginServerCommUseCaseWithCallbacks(
		g.usc.playerManager,
		g.usc.serverConfig,
		g.SetAuthenticated, // Callback to update authentication status
		g.SendAuthRequest,  // Callback to send AuthRequest after InitLS
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

func (g *GameServer) sendAuthRequest(ctx context.Context) error {
	log.Ctx(ctx).Info().Msg("Sending AuthRequest to LoginServer")

	// TODO: Generate proper hex ID for server authentication
	// For now, use a simple placeholder
	hexID := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	// TODO: Configure proper subnet/host lists
	subnets := []string{"127.0.0.0/8"}
	hosts := []string{"127.0.0.1"}

	extIP := net.ParseIP(g.config.externalIP)
	subnets = append(subnets, "0.0.0.0/0")
	hosts = append(hosts, extIP.String())

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

	g.prepareUseCases()
	g.prepareHandlers()

	// Connect to LoginServer after database is ready (with retry logic)
	if err := g.connectToLoginServerWithRetry(ctx); err != nil {
		return fmt.Errorf("LoginServer connection setup failed: %w", err)
	}

	return g.run(ctx)
}

func (g *GameServer) run(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	// Start heartbeat routine
	eg.Go(func() error {
		g.startHeartbeat(ctx)
		return nil
	})

	// Start world cleanup routine
	eg.Go(func() error {
		g.startWorldCleanup(ctx)
		return nil
	})

	// Start game loop
	eg.Go(func() error {
		return g.gameLoop.Run(ctx)
	})

	// Start game client listener
	eg.Go(func() error {
		return clienttransport.ListenAndServe(ctx, g.config.gameListener(), g.handlers.client.Handle)
	})

	// Graceful shutdown
	eg.Go(func() error {
		<-ctx.Done()

		log.Ctx(ctx).Info().Msg("GameServer shutting down...")

		// Disconnect from LoginServer
		if g.loginServerHandler != nil {
			g.loginServerHandler.Disconnect()
		}

		// Close database connection
		if g.db != nil {
			g.db.Close()
			log.Ctx(ctx).Info().Msg("Database connection closed")
		}

		// TODO: Close client handlers
		// g.handlers.client.Close()

		log.Ctx(ctx).Info().Msg("GameServer shutdown complete")
		return nil
	})

	return eg.Wait()
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
