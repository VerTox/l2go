package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver"
	"github.com/VerTox/l2go/internal/loginserver/schema"
	"github.com/VerTox/l2go/pkg/gracel"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx = log.With().Str("service", "loginserver").Logger().WithContext(ctx)
	log.Ctx(ctx).Info().Msg("Starting loginserver")

	count, err := schema.Up(get().Database.String())
	if err != nil {
		log.Ctx(ctx).Panic().Err(err).Msg("unable to run migrations")
	}

	log.Ctx(ctx).Info().Int("count", count).Msg("Migrations loaded")

	db, err := pgxpool.New(ctx, get().Database.String())
	if err != nil {
		log.Ctx(ctx).Panic().Err(err).Msg("failed connect to db")
	}
	defer db.Close()

	loginParams := loginserver.Params{
		DB:             db,
		LoginHost:      get().LoginServer.Host,
		LoginPort:      get().LoginServer.Port,
		GameServerPort: get().LoginServer.GameServerPort,
	}

	service := loginserver.New(loginParams)

	gr := gracel.NewGracel(service, nil)

	log.Ctx(ctx).Info().Msg("Service started")

	if err := gr.Run(ctx); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("gracel run failed")
	}

	log.Ctx(ctx).Info().Msg("Service stopped gracefully")
}
