package store

import (
	"time"

	"comp-health/internal/model"
)

// Store is the interface that both MemoryStore and future persistent stores must satisfy.
type Store interface {
	SaveResult(model.CheckResult)
	SaveReport(model.NodeReport)
	Snapshot(rangeWindow time.Duration, points int) []model.ServiceStatus
	Nodes() map[string]time.Time
	Close() error
}
