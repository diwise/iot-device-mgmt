package database

import (
	"context"
	"fmt"
	"os"
	"time"

	"log/slog"

	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
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

func LoadConfigFromEnv(ctx context.Context) ConnectorConfig {
	dbHost := os.Getenv("POSTGRES_HOST")
	username := os.Getenv("POSTGRES_USER")
	dbName := os.Getenv("POSTGRES_DBNAME")
	password := os.Getenv("POSTGRES_PASSWORD")
	sslMode := env.GetVariableOrDefault(ctx, "POSTGRES_SSLMODE", "disable")

	return ConnectorConfig{
		Host:     dbHost,
		Username: username,
		DbName:   dbName,
		Password: password,
		SslMode:  sslMode,
	}
}

type ConnectorFunc func() (*gorm.DB, error)

func NewSQLiteConnector(ctx context.Context) ConnectorFunc {
	return func() (*gorm.DB, error) {
		db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
			Logger:          logger.Default.LogMode(logger.Silent),
			CreateBatchSize: 1000,
		})

		if err == nil {
			db.Exec("PRAGMA foreign_keys = ON")
			sqldb, _ := db.DB()
			sqldb.SetMaxOpenConns(1)
		}

		return db, err
	}
}

func NewPostgreSQLConnector(ctx context.Context, cfg ConnectorConfig) ConnectorFunc {
	dbHost := cfg.Host
	username := cfg.Username
	dbName := cfg.DbName
	password := cfg.Password
	sslMode := cfg.SslMode

	dbURI := fmt.Sprintf("host=%s user=%s dbname=%s sslmode=%s password=%s", dbHost, username, dbName, sslMode, password)

	log := logging.GetFromContext(ctx)

	return func() (*gorm.DB, error) {
		sublogger := log.With(
			slog.String("host", dbHost),
			slog.String("database", dbName),
		)

		for {
			sublogger.Info("connecting to database host")

			db, err := gorm.Open(postgres.Open(dbURI), &gorm.Config{
				Logger: logger.New(
					&logadapter{logger: sublogger},
					logger.Config{
						SlowThreshold:             time.Second,
						LogLevel:                  logger.Info,
						IgnoreRecordNotFoundError: false,
						Colorful:                  false,
					},
				),
			})
			if err != nil {
				sublogger.Error("failed to connect to database", "err", err.Error())
				time.Sleep(3 * time.Second)
				os.Exit(1)
			} else {
				return db, nil
			}
		}
	}
}

// logadapter provides a Printf interface to the gorm logger
// so that we can forward the log data to slog
type logadapter struct {
	logger *slog.Logger
}

func (adapter *logadapter) Printf(format string, args ...interface{}) {
	adapter.logger.Info(fmt.Sprintf(format, args...))
}
