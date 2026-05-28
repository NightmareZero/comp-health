package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"comp-health/internal/config"
	"comp-health/internal/model"
	"comp-health/internal/probe"
)

type Callback func(model.CheckResult)

type Scheduler struct {
	registry *probe.Registry
}

func New(registry *probe.Registry) *Scheduler {
	return &Scheduler{registry: registry}
}

func (s *Scheduler) Start(ctx context.Context, probes []config.Probe, callback Callback) {
	var wg sync.WaitGroup
	for _, item := range probes {
		probeCfg := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			if probeCfg.Interval <= 0 {
				probeCfg.Interval = 30 * time.Second
			}
			ticker := time.NewTicker(probeCfg.Interval)
			defer ticker.Stop()

			s.runOnce(ctx, probeCfg, callback)
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.runOnce(ctx, probeCfg, callback)
				}
			}
		}()
	}
	<-ctx.Done()
	wg.Wait()
}

func (s *Scheduler) runOnce(ctx context.Context, probeCfg config.Probe, callback Callback) {
	probeCtx, cancel := context.WithTimeout(ctx, probeCfg.Timeout)
	defer cancel()
	result, err := s.registry.Run(probeCtx, probeCfg)
	if err != nil {
		log.Printf("probe %s failed to run: %v", probeCfg.ID, err)
		result = model.CheckResult{
			ProbeID:   probeCfg.ID,
			Name:      probeCfg.Name,
			Type:      probeCfg.Type,
			Target:    probeCfg.Target,
			Status:    model.StatusDown,
			Message:   err.Error(),
			CheckedAt: time.Now(),
		}
	}
	callback(result)
}
