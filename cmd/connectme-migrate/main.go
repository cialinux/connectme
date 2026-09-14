package main

import (
	"context"
	"github.com/connectme/connectme/core/config"
	"github.com/connectme/connectme/core/database"
	"github.com/connectme/connectme/migrations"
	"log/slog"
	"os"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	pool, err := database.Open(ctx, cfg.Database)
	if err != nil {
		slog.Error("database failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	for _, name := range []string{"core", "audit", "identity", "authorization", "locations", "hosts", "credentials", "connections"} {
		ms, loadErr := database.LoadMigrations(migrations.FS, name, name)
		if loadErr != nil {
			err = loadErr
			break
		}
		if err = database.Migrate(ctx, pool, ms); err != nil {
			break
		}
	}
	if err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")
}
