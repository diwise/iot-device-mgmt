package storage

import (
	"context"
	_ "embed"
	"errors"
	"sync"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrStoreFailed          = errors.New("could not store data")
	ErrNoID                 = errors.New("data contains no id")
	ErrMissingTenant        = errors.New("missing tenant information")
	ErrStatusDeviceNotFound = errors.New("device not found for status message")
)

//go:embed migrate.sql
var migrateSQL string

type Storage struct {
	conn *pgxpool.Pool
	mu   sync.Mutex
}

func New(ctx context.Context, config Config) (*Storage, error) {
	pool, err := newPool(ctx, config)
	if err != nil {
		return nil, err
	}

	s := &Storage{
		conn: pool,
	}

	return s, initialize(ctx, s)
}

func newPool(ctx context.Context, config Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(config.ConnStr())
	if err != nil {
		return nil, err
	}

	poolConfig.MaxConns = 10
	poolConfig.MinConns = 0
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "iot-device-mgmt"

	p, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}

	err = p.Ping(ctx)
	if err != nil {
		p.Close()
		return nil, err
	}

	return p, nil
}

func initialize(ctx context.Context, s *Storage) error {
	_, err := s.conn.Exec(ctx, migrateSQL)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) Close() {
	s.conn.Close()
}

func (s *Storage) GetTenants(ctx context.Context) (types.Collection[string], error) {
	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return types.Collection[string]{}, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, "SELECT DISTINCT tenant FROM devices ORDER BY tenant ASC", nil)
	if err != nil {
		return types.Collection[string]{}, err
	}
	defer rows.Close()

	tenants := []string{}

	for rows.Next() {
		var tenant string

		err := rows.Scan(&tenant)
		if err != nil {
			return types.Collection[string]{}, err
		}

		if tenant != "" {
			tenants = append(tenants, tenant)
		}
	}

	if err := rows.Err(); err != nil {
		return types.Collection[string]{}, err
	}

	return types.Collection[string]{
		Data:       tenants,
		Count:      uint64(len(tenants)),
		Offset:     0,
		Limit:      uint64(len(tenants)),
		TotalCount: uint64(len(tenants)),
	}, nil
}
