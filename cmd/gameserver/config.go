package main

import (
	"fmt"
	"sync"

	"github.com/kelseyhightower/envconfig"
)

var (
	instance Config
	once     sync.Once
)

// getConfig возвращает экземпляр конфигурации сервиса.
func getConfig() Config {
	once.Do(func() {
		if err := envconfig.Process("", &instance); err != nil {
			panic(fmt.Errorf("process: %w", err))
		}
	})

	return instance
}

type Config struct {
	GameServer  gameServer
	LoginServer loginServerConnection
	Database    databaseCfg
	Network     networkCfg
}

type gameServer struct {
	ServerID     int     `envconfig:"GAME_SERVER_ID" default:"1"`
	ServerName   string  `envconfig:"GAME_SERVER_NAME" default:"L2Go Bartz"`
	ServerPort   int     `envconfig:"GAME_SERVER_PORT" default:"7777"`
	MaxPlayers   int     `envconfig:"GAME_SERVER_MAX_PLAYERS" default:"1000"`
	ServerType   int     `envconfig:"GAME_SERVER_TYPE" default:"1"` // 1=Normal, 2=Relax, 4=Test
	MinLevel     int     `envconfig:"GAME_SERVER_MIN_LEVEL" default:"1"`
	MaxLevel     int     `envconfig:"GAME_SERVER_MAX_LEVEL" default:"85"`
	AgeLimit     int     `envconfig:"GAME_SERVER_AGE_LIMIT" default:"0"`
	ShowBrackets bool    `envconfig:"GAME_SERVER_SHOW_BRACKETS" default:"false"`
	PvP          bool    `envconfig:"GAME_SERVER_PVP" default:"true"`
	TestServer   bool    `envconfig:"GAME_SERVER_TEST" default:"false"`
	ShowClock    bool    `envconfig:"GAME_SERVER_SHOW_CLOCK" default:"true"`
	ExpRate      float64 `envconfig:"GAME_SERVER_EXP_RATE" default:"1.0"`
	SpRate       float64 `envconfig:"GAME_SERVER_SP_RATE" default:"1.0"`
}

type loginServerConnection struct {
	Host string `envconfig:"LOGIN_SERVER_HOST" default:"127.0.0.1"`
	Port string `envconfig:"LOGIN_SERVER_PORT" default:"9014"`
}

type networkCfg struct {
	GameHost string `envconfig:"GAME_HOST" default:"0.0.0.0"`
	GamePort string `envconfig:"GAME_PORT" default:"7777"`
	External string `envconfig:"EXTERNAL_HOST" default:"127.0.0.1"`
}

type databaseCfg struct {
	// MongoDB configuration (legacy - for future use)
	MongoHost     string `envconfig:"MONGO_HOST" default:"127.0.0.1"`
	MongoPort     string `envconfig:"MONGO_PORT" default:"27017"`
	MongoDatabase string `envconfig:"MONGO_DATABASE" default:"l2go_gameserver"`
	MongoUsername string `envconfig:"MONGO_USERNAME"`
	MongoPassword string `envconfig:"MONGO_PASSWORD"`

	// PostgreSQL configuration (future migration)
	PostgresHost     string `envconfig:"POSTGRES_HOST" default:"127.0.0.1"`
	PostgresPort     string `envconfig:"POSTGRES_PORT" default:"5432"`
	PostgresUsername string `envconfig:"POSTGRES_USERNAME" default:"postgres"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD" default:"postgres"`
	PostgresDatabase string `envconfig:"POSTGRES_DATABASE" default:"l2go_gameserver"`
}

func (c databaseCfg) MongoConnectionString() string {
	if c.MongoUsername != "" && c.MongoPassword != "" {
		return fmt.Sprintf(
			"mongodb://%s:%s@%s:%s/%s",
			c.MongoUsername,
			c.MongoPassword,
			c.MongoHost,
			c.MongoPort,
			c.MongoDatabase,
		)
	}
	return fmt.Sprintf(
		"mongodb://%s:%s/%s",
		c.MongoHost,
		c.MongoPort,
		c.MongoDatabase,
	)
}

func (c databaseCfg) PostgresConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.PostgresUsername,
		c.PostgresPassword,
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDatabase,
	)
}
