package probe

import (
	"context"
	"fmt"

	"comp-health/internal/config"
	"comp-health/internal/model"
)

type Adapter interface {
	Type() string
	Run(ctx context.Context, cfg config.Probe) model.CheckResult
}

type Registry struct {
	adapters map[string]Adapter
}

func NewRegistry(adapters ...Adapter) *Registry {
	items := make(map[string]Adapter, len(adapters))
	for _, adapter := range adapters {
		items[adapter.Type()] = adapter
	}
	return &Registry{adapters: items}
}

func (r *Registry) Run(ctx context.Context, cfg config.Probe) (model.CheckResult, error) {
	adapter, ok := r.adapters[cfg.Type]
	if !ok {
		return model.CheckResult{}, fmt.Errorf("unsupported probe type: %s", cfg.Type)
	}
	return adapter.Run(ctx, cfg), nil
}
