package main

import (
	"context"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver"
	"github.com/VerTox/l2go/pkg/gracel"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx = log.With().Str("service", "gameserver").Logger().WithContext(ctx)
	log.Ctx(ctx).Info().Msg("Starting gameserver")

	// Load configuration from environment
	config := getConfig()

	// Create GameServer with configuration from environment
	gameServerParams := gameserver.Params{
		// Server configuration
		ServerID:   config.GameServer.ServerID,
		ServerName: config.GameServer.ServerName,
		ServerPort: config.GameServer.ServerPort,
		MaxPlayers: config.GameServer.MaxPlayers,

		// LoginServer connection
		LoginServerHost: config.LoginServer.Host,
		LoginServerPort: config.LoginServer.Port,

		// Network configuration
		GameHost:   config.Network.GameHost,
		GamePort:   config.Network.GamePort,
		ExternalIP: config.Network.External,

		// Database configuration
		DatabaseURL: config.Database.PostgresConnectionString(),

		// Server properties
		ServerType:   config.GameServer.ServerType,
		MinLevel:     config.GameServer.MinLevel,
		MaxLevel:     config.GameServer.MaxLevel,
		AgeLimit:     config.GameServer.AgeLimit,
		ShowBrackets: config.GameServer.ShowBrackets,
		PvP:          config.GameServer.PvP,
		TestServer:   config.GameServer.TestServer,
		ShowClock:    config.GameServer.ShowClock,
		ExpRate:      config.GameServer.ExpRate,
		SpRate:       config.GameServer.SpRate,
	}

	service := gameserver.New(gameServerParams)

	gr := gracel.NewGracel(service, nil)

	log.Ctx(ctx).Info().
		Int("server_id", config.GameServer.ServerID).
		Str("server_name", config.GameServer.ServerName).
		Str("login_server", config.LoginServer.Host+":"+config.LoginServer.Port).
		Str("game_port", strconv.Itoa(config.GameServer.ServerPort)).
		Msg("Service started")

	if err := gr.Run(ctx); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("gracel run failed")
	}

	log.Ctx(ctx).Info().Msg("Service stopped gracefully")
}
