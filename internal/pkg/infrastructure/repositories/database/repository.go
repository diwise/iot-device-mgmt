package database

import (
	"fmt"
	"os"
	"time"

	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/rs/zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ConnectorConfig struct {
	Host     string
	Username string
	DbName   string
	Password string
	SslMode  string
}

func LoadConfigFromEnv(log zerolog.Logger) ConnectorConfig {
	dbHost := os.Getenv("POSTGRES_HOST")
	username := os.Getenv("POSTGRES_USER")
	dbName := os.Getenv("POSTGRES_DBNAME")
	password := os.Getenv("POSTGRES_PASSWORD")
	sslMode := env.GetVariableOrDefault(log, "POSTGRES_SSLMODE", "disable")

	return ConnectorConfig{
		Host:     dbHost,
		Username: username,
		DbName:   dbName,
		Password: password,
		SslMode:  sslMode,
	}
}

type ConnectorFunc func() (*gorm.DB, zerolog.Logger, error)

func NewSQLiteConnector(log zerolog.Logger) ConnectorFunc {
	return func() (*gorm.DB, zerolog.Logger, error) {
		db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
			Logger:          logger.Default.LogMode(logger.Silent),
			CreateBatchSize: 1000,
		})

		if err == nil {
			db.Exec("PRAGMA foreign_keys = ON")
			sqldb, _ := db.DB()
			sqldb.SetMaxOpenConns(1)
		}

		return db, log, err
	}
}

func NewPostgreSQLConnector(log zerolog.Logger, cfg ConnectorConfig) ConnectorFunc {
	dbHost := cfg.Host
	username := cfg.Username
	dbName := cfg.DbName
	password := cfg.Password
	sslMode := cfg.SslMode

	dbURI := fmt.Sprintf("host=%s user=%s dbname=%s sslmode=%s password=%s", dbHost, username, dbName, sslMode, password)

	return func() (*gorm.DB, zerolog.Logger, error) {
		sublogger := log.With().Str("host", dbHost).Str("database", dbName).Logger()

		for {
			sublogger.Info().Msg("connecting to database host")

			db, err := gorm.Open(postgres.Open(dbURI), &gorm.Config{
				Logger: logger.New(
					&sublogger,
					logger.Config{
						SlowThreshold:             time.Second,
						LogLevel:                  logger.Info,
						IgnoreRecordNotFoundError: false,
						Colorful:                  false,
					},
				),
			})
			if err != nil {
				sublogger.Fatal().Err(err).Msg("failed to connect to database")
				time.Sleep(3 * time.Second)
			} else {
				return db, sublogger, nil
			}
		}
	}
}
