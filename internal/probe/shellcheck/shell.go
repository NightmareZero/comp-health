package shellcheck

import (
	"context"
	"os/exec"
	"runtime"
	"time"

	"comp-health/internal/config"
	"comp-health/internal/model"
)

type Adapter struct{}

func New() Adapter {
	return Adapter{}
}

func (Adapter) Type() string {
	return "shell"
}

func (Adapter) Run(ctx context.Context, cfg config.Probe) model.CheckResult {
	started := time.Now()
	result := model.CheckResult{
		ProbeID:   cfg.ID,
		Name:      cfg.Name,
		Type:      cfg.Type,
		Target:    cfg.Command,
		CheckedAt: started,
		Status:    model.StatusDown,
	}

	command := exec.CommandContext(ctx, shellName(), shellArgs(cfg.Command)...) // #nosec G204
	if len(cfg.Env) > 0 {
		for k, v := range cfg.Env {
			command.Env = append(command.Env, k+"="+v)
		}
	}
	output, err := command.CombinedOutput()
	result.LatencyMS = time.Since(started).Milliseconds()
	result.Metadata = map[string]string{"output": string(output)}
	if err != nil {
		result.Message = err.Error()
		return result
	}
	result.Status = model.StatusUp
	result.Message = "command succeeded"
	return result
}

func shellName() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
}

func shellArgs(command string) []string {
	if runtime.GOOS == "windows" {
		return []string{"/C", command}
	}
	return []string{"-c", command}
}
