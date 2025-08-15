package loginserver

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/VerTox/l2go/internal/loginserver/events"
	"github.com/VerTox/l2go/internal/loginserver/handlers/client"
	"github.com/VerTox/l2go/internal/loginserver/handlers/gameserver"
	"github.com/VerTox/l2go/internal/loginserver/models"
	"github.com/VerTox/l2go/internal/loginserver/registry"
	"github.com/VerTox/l2go/internal/loginserver/repo"
	"github.com/VerTox/l2go/internal/loginserver/usecase"
)

type LoginServer struct {
	db                     *pgxpool.Pool
	repo                   *repo.Repository
	gameServerRegistry     registry.GameServerRegistry
	characterCountRegistry registry.CharacterCountRegistry
	eventBus               events.EventBus
	internalServersList    []byte
	externalServersList    []byte
	status                 loginServerStatus
	clientsListener        net.Listener
	gameServersListener    net.Listener

	config   config
	handlers handlers
	usc      usecases
}

type usecases struct {
	client         *usecase.ClientUseCase
	gameServer     *usecase.GameServerUseCase
	gameServerComm *usecase.GameServerCommUseCase
	session        *usecase.SessionUseCase
}

type handlers struct {
	client     *client.Handler
	gameServer *gameserver.Handler
}

type Params struct {
	DB             *pgxpool.Pool
	LoginHost      string
	LoginPort      string
	GameServerPort string
}

type config struct {
	host string
	port string

	gameServerPort string
}

func (c *config) clientListener() string {
	return fmt.Sprintf("%s:%s", c.host, c.port)
}

func (c *config) gameServerListener() string {
	return fmt.Sprintf("%s:%s", c.host, c.gameServerPort)
}

type loginServerStatus struct {
	successfulAccountCreation uint32
	failedAccountCreation     uint32
	successfulLogins          uint32
	failedLogins              uint32
	hackAttempts              uint32
}

func (l *LoginServer) prepareRepositories() {
	l.repo = repo.NewRepository(l.db)
}

func (l *LoginServer) prepareRegistry() {
	l.gameServerRegistry = registry.NewGameServerRegistry()
	l.characterCountRegistry = registry.NewCharacterCountRegistry(5 * time.Minute) // 5 minute cache TTL (increased since we request earlier)
	l.eventBus = events.NewEventBus()
}

func (l *LoginServer) prepareUseCases() {
	// Initialize session use case with 5 minute session timeout
	l.usc.session = usecase.NewSessionUseCase(5 * time.Minute)

	l.usc.client = usecase.NewClientUseCase(usecase.Params{
		Repo:              *l.repo,
		AutoCreateAccount: true,
		SessionUseCase:    l.usc.session,
	})

	l.usc.gameServer = usecase.NewGameServerUseCase(usecase.GameServerUseCaseParams{
		Registry: l.gameServerRegistry,
	})

	// Create GameServerCommUseCase with EventBus
	l.usc.gameServerComm = usecase.NewGameServerCommUseCase(usecase.GameServerCommParams{
		GameServerRegistry:     l.gameServerRegistry,
		SessionUseCase:         l.usc.session,
		CharacterCountRegistry: l.characterCountRegistry,
		EventBus:               l.eventBus,
	})
}

func (l *LoginServer) prepareHandlers() {
	l.handlers = handlers{
		client:     client.New(l.clientsListener, l.usc.client, l.usc.gameServer, l.usc.gameServerComm),
		gameServer: gameserver.New(l.gameServersListener, l.usc.gameServerComm),
	}

	// Subscribe GameServer handler to EventBus events
	l.eventBus.Subscribe("send_packet", l.handleSendPacketEvent)
}

func (l *LoginServer) prepareListeners(ctx context.Context) error {
	var err error

	// Listen for inclient connections
	l.clientsListener, err = net.Listen("tcp", l.config.clientListener())
	if err != nil {
		return fmt.Errorf("couldn't initialize the Login Server (Clients listener): %w", err)
	}
	log.Ctx(ctx).Info().Msgf("Login Server listening for clients connections on port %s", l.config.port)

	// Listen for game servers connections
	l.gameServersListener, err = net.Listen("tcp", l.config.gameServerListener())
	if err != nil {
		return fmt.Errorf("couldn't initialize the Login Server (Gameservers listener): %w", err)
	}
	log.Ctx(ctx).Info().Msgf("Login Server listening for gameservers connections on port %s", l.config.gameServerPort)

	return nil
}

func New(p Params) *LoginServer {
	return &LoginServer{
		db: p.DB,
		config: config{
			host:           p.LoginHost,
			port:           p.LoginPort,
			gameServerPort: p.GameServerPort,
		},
	}
}

func (l *LoginServer) Run(ctx context.Context) error {
	l.prepareRepositories()
	l.prepareRegistry()

	if err := l.prepareListeners(ctx); err != nil {
		return fmt.Errorf("prepare listeners: %w", err)
	}

	l.prepareUseCases()
	l.prepareHandlers()

	// Register demo game servers for testing
	//if err := l.registerDemoServers(ctx); err != nil {
	//	log.Ctx(ctx).Warn().Err(err).Msg("Failed to register demo servers")
	//}

	return l.run(ctx)
}

func (l *LoginServer) run(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	// Start client handler
	eg.Go(func() error {
		l.handlers.client.ListenAndServe(ctx)
		return nil
	})

	// Start GameServer handler
	eg.Go(func() error {
		l.handlers.gameServer.ListenAndServe(ctx)
		return nil
	})

	// Graceful shutdown
	eg.Go(func() error {
		<-ctx.Done()

		l.handlers.client.Close()
		l.handlers.gameServer.Close()

		return nil
	})

	return eg.Wait()
}

// generateRandomUint32 generates a random uint32 value
func generateRandomUint32() uint32 {
	var bytes [4]byte
	rand.Read(bytes[:])
	return uint32(bytes[0]) | (uint32(bytes[1]) << 8) | (uint32(bytes[2]) << 16) | (uint32(bytes[3]) << 24)
}

// handleSendPacketEvent handles SendPacketEvent from EventBus
func (l *LoginServer) handleSendPacketEvent(ctx context.Context, event events.Event) error {
	sendPacketEvent, ok := event.(*events.SendPacketEvent)
	if !ok {
		return fmt.Errorf("invalid event type, expected SendPacketEvent")
	}

	// Forward the packet to GameServer handler
	err := l.handlers.gameServer.SendToServer(ctx, sendPacketEvent.ServerID, sendPacketEvent.Data)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).
			Int("server_id", sendPacketEvent.ServerID).
			Msg("Failed to send packet via EventBus")
		return err
	}

	log.Ctx(ctx).Debug().
		Int("server_id", sendPacketEvent.ServerID).
		Int("data_length", len(sendPacketEvent.Data)).
		Msg("Packet sent successfully via EventBus")

	return nil
}

// registerDemoServers registers demo game servers for testing
func (l *LoginServer) registerDemoServers(ctx context.Context) error {
	demoServers := []*models.GameServerInfo{
		{
			ID:             1,
			Name:           "Bartz",
			Port:           7777,
			Status:         models.ServerStatusOnline,
			CurrentPlayers: 0,
			MaxPlayers:     1000,
			PvP:            true,
			AgeLimit:       0,
			ServerType:     1, // Normal
			ShowBrackets:   false,
		},
		{
			ID:             2,
			Name:           "Sieghardt",
			Port:           7778,
			Status:         models.ServerStatusOnline,
			CurrentPlayers: 256,
			MaxPlayers:     800,
			PvP:            false,
			AgeLimit:       0,
			ServerType:     2, // Relax
			ShowBrackets:   true,
		},
		{
			ID:             3,
			Name:           "Test Server",
			Port:           7779,
			Status:         models.ServerStatusTest,
			CurrentPlayers: 5,
			MaxPlayers:     100,
			PvP:            true,
			AgeLimit:       0,
			ServerType:     4, // Test
			ShowBrackets:   false,
		},
	}

	// Add default addresses for demo servers
	for _, server := range demoServers {
		server.AddServerAddress("127.0.0.0/8", "127.0.0.1") // Localhost clients
		server.AddServerAddress("0.0.0.0/0", "127.0.0.1")   // Default/fallback
	}

	for _, server := range demoServers {
		if err := l.usc.gameServer.RegisterGameServer(ctx, server); err != nil {
			log.Ctx(ctx).Error().Err(err).Int("server_id", server.ID).Msg("Failed to register demo server")
			continue
		}
		log.Ctx(ctx).Info().Int("server_id", server.ID).Str("name", server.Name).Msg("Registered demo game server")
	}

	return nil
}
