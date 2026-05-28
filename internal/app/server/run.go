package server

import (
	"context"
	"fmt"
	"log"

	"comp-health/internal/config"
	"comp-health/internal/model"
	"comp-health/internal/probe"
	"comp-health/internal/probe/httpcheck"
	"comp-health/internal/probe/shellcheck"
	"comp-health/internal/probe/tcpcheck"
	"comp-health/internal/scheduler"
	httpserver "comp-health/internal/server"
	"comp-health/internal/store"
	"comp-health/internal/webfs"
)

func Run(ctx context.Context, cfg *config.Config) error {
	storage, err := newStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer storage.Close()

	registry := probe.NewRegistry(
		httpcheck.New(),
		shellcheck.New(),
		tcpcheck.New(),
	)

	if cfg.Server.EnableLocalProbe && len(cfg.Probes) > 0 {
		s := scheduler.New(registry)
		go s.Start(ctx, cfg.Probes, func(result model.CheckResult) {
			storage.SaveResult(result)
		})
	}

	srv := httpserver.New(cfg, storage, webfs.FS)
	log.Printf("server listening on %s", cfg.Server.Listen)
	return srv.Run(ctx)
}

func newStore(ctx context.Context, cfg *config.Config) (store.Store, error) {
	switch cfg.Storage.Driver {
	case "sqlite":
		return store.NewSQLiteStore(ctx, cfg.Storage.Path, cfg.Server.Retention, cfg.Storage.CleanupInterval)
	case "memory", "":
		return store.NewMemoryStore(cfg.Server.Retention), nil
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", cfg.Storage.Driver)
	}
}
