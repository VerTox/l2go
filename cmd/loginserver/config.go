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

// Get возвращает экземпляр конфигурации сервиса.
func get() Config {
	once.Do(func() {
		if err := envconfig.Process("", &instance); err != nil {
			panic(fmt.Errorf("process: %w", err))
		}
	})

	return instance
}

type Config struct {
	LoginServer loginServer
	Database    postgresCfg
}

type loginServer struct {
	Host           string `envconfig:"LOGIN_SERVER_HOST" default:"0.0.0.0"`
	Port           string `envconfig:"LOGIN_SERVER_PORT" default:"2106"`
	GameServerPort string `envconfig:"GAME_SERVER_PORT" default:"9014"`
	AutoCreate     bool   `envconfig:"AUTO_CREATE_ACCOUNTS" default:"true"`
}

type postgresCfg struct {
	Host         string `envconfig:"POSTGRES_HOST" required:"true"`
	Port         string `envconfig:"POSTGRES_PORT" required:"true"`
	Username     string `envconfig:"POSTGRES_USERNAME" required:"true"`
	Password     string `envconfig:"POSTGRES_PASSWORD" required:"true"`
	Database     string `envconfig:"POSTGRES_DATABASE" required:"true"`
	MigrationDir string `envconfig:"POSTGRES_MIGRATION_DIR" required:"true"`
}

func (c postgresCfg) String() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.Username,
		c.Password,
		c.Host,
		c.Port,
		c.Database,
	)
}
