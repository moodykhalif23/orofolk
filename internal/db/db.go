package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig tunes the connection pool. Non-positive values fall back to safe
// defaults (20 max connections, 5m idle recycle).
type PoolConfig struct {
	MaxConns        int32
	MaxConnIdleTime time.Duration
}

// NewPool creates and verifies a pgx connection pool with default tuning.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return NewPoolWithConfig(ctx, dsn, PoolConfig{})
}

// NewPoolWithConfig creates and verifies a pgx connection pool with the given
// tuning. The pool is configurable so production can raise connection limits
// beyond the conservative default.
func NewPoolWithConfig(ctx context.Context, dsn string, pc PoolConfig) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if pc.MaxConns <= 0 {
		pc.MaxConns = 20
	}
	if pc.MaxConnIdleTime <= 0 {
		pc.MaxConnIdleTime = 5 * time.Minute
	}
	cfg.MaxConns = pc.MaxConns
	cfg.MaxConnIdleTime = pc.MaxConnIdleTime
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}
