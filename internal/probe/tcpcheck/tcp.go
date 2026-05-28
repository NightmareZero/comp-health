package tcpcheck

import (
	"context"
	"fmt"
	"net"
	"time"

	"comp-health/internal/config"
	"comp-health/internal/model"
)

// Adapter is the probe.Adapter implementation for TCP connectivity checks.
type Adapter struct{}

func New() Adapter { return Adapter{} }

func (Adapter) Type() string { return "tcp" }

func (Adapter) Run(ctx context.Context, cfg config.Probe) model.CheckResult {
	started := time.Now()
	result := model.CheckResult{
		ProbeID:   cfg.ID,
		Name:      cfg.Name,
		Type:      cfg.Type,
		Target:    cfg.Target,
		CheckedAt: started,
		Status:    model.StatusDown,
	}

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", cfg.Target)
	result.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		result.Message = err.Error()
		return result
	}
	conn.Close()
	result.Status = model.StatusUp
	result.Message = fmt.Sprintf("TCP connected to %s", cfg.Target)
	return result
}
