package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"comp-health/internal/config"
	"comp-health/internal/model"
	"comp-health/internal/probe"
	"comp-health/internal/probe/httpcheck"
	"comp-health/internal/probe/shellcheck"
	"comp-health/internal/probe/tcpcheck"
	"comp-health/internal/report"
	"comp-health/internal/scheduler"
)

func Run(ctx context.Context, cfg *config.Config) error {
	registry := probe.NewRegistry(
		httpcheck.New(),
		shellcheck.New(),
		tcpcheck.New(),
	)
	client := report.NewClient(cfg.Agent.ServerURL, cfg.Agent.Token)

	results := make(map[string]model.CheckResult)
	var mu sync.RWMutex

	s := scheduler.New(registry)
	go s.Start(ctx, cfg.Probes, func(result model.CheckResult) {
		result.NodeID = cfg.Agent.NodeID
		result.NodeName = cfg.Agent.NodeName
		mu.Lock()
		results[result.ProbeID] = result
		mu.Unlock()
	})

	ticker := time.NewTicker(cfg.Agent.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			reportData := snapshot(cfg, results, &mu)
			if len(reportData.Results) == 0 {
				continue
			}
			if err := client.Send(ctx, reportData); err != nil {
				log.Printf("report send failed: %v", err)
			}
		}
	}
}

func snapshot(cfg *config.Config, current map[string]model.CheckResult, mu *sync.RWMutex) model.NodeReport {
	mu.RLock()
	defer mu.RUnlock()
	results := make([]model.CheckResult, 0, len(current))
	for _, result := range current {
		results = append(results, result)
	}
	return model.NodeReport{
		NodeID:     cfg.Agent.NodeID,
		NodeName:   cfg.Agent.NodeName,
		ReportedAt: time.Now(),
		Results:    results,
	}
}
