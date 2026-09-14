package database

import (
	"context"
	"fmt"
	"github.com/connectme/connectme/core/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

func Open(ctx context.Context, cfg config.Database) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	pc.MaxConns = cfg.MaxConnections
	pc.MinConns = cfg.MinConnections
	pc.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err = pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}
func Healthy(ctx context.Context, p *pgxpool.Pool) error {
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return p.Ping(c)
}
