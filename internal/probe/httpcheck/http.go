package httpcheck

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"comp-health/internal/config"
	"comp-health/internal/model"
)

type Adapter struct{}

func New() Adapter {
	return Adapter{}
}

func (Adapter) Type() string {
	return "http"
}

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

	req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.Target, nil)
	if err != nil {
		result.Message = err.Error()
		return result
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		result.Message = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.LatencyMS = time.Since(started).Milliseconds()
	expect := cfg.ExpectStatus
	if expect == 0 {
		expect = http.StatusOK
	}
	if resp.StatusCode == expect {
		result.Status = model.StatusUp
		result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return result
	}
	result.Message = fmt.Sprintf("unexpected status: %d", resp.StatusCode)
	return result
}
