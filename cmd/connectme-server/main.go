package main

import (
	"context"
	"github.com/connectme/connectme/core/bootstrap"
	"github.com/connectme/connectme/core/config"
	"github.com/connectme/connectme/core/logging"
	"github.com/connectme/connectme/migrations"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		c := http.Client{Timeout: 2 * time.Second}
		resp, err := c.Get("http://127.0.0.1:8080/livez")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		_ = resp.Body.Close()
		return
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	log := logging.New(cfg.Environment)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	app, err := bootstrap.Build(ctx, cfg, log, migrations.FS)
	if err != nil {
		log.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
	if err = app.Run(ctx); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}
