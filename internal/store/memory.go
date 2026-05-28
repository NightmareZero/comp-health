package store

import (
	"sort"
	"sync"
	"time"

	"comp-health/internal/model"
)

type MemoryStore struct {
	mu       sync.RWMutex
	latest   map[string]model.CheckResult
	history  map[string][]model.StatusPoint
	nodes    map[string]time.Time
	nodeName map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		latest:   make(map[string]model.CheckResult),
		history:  make(map[string][]model.StatusPoint),
		nodes:    make(map[string]time.Time),
		nodeName: make(map[string]string),
	}
}

func (s *MemoryStore) SaveResult(result model.CheckResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := resultKey(result.NodeID, result.ProbeID)
	s.latest[key] = result
	s.history[key] = append(s.history[key], model.StatusPoint{At: result.CheckedAt, Status: result.Status})
	if len(s.history[key]) > 90 {
		s.history[key] = s.history[key][len(s.history[key])-90:]
	}
	if result.NodeID != "" {
		s.nodes[result.NodeID] = result.CheckedAt
		s.nodeName[result.NodeID] = result.NodeName
	}
}

func (s *MemoryStore) SaveReport(report model.NodeReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[report.NodeID] = report.ReportedAt
	s.nodeName[report.NodeID] = report.NodeName
	for _, result := range report.Results {
		key := resultKey(report.NodeID, result.ProbeID)
		result.NodeID = report.NodeID
		result.NodeName = report.NodeName
		s.latest[key] = result
		s.history[key] = append(s.history[key], model.StatusPoint{At: result.CheckedAt, Status: result.Status})
		if len(s.history[key]) > 90 {
			s.history[key] = s.history[key][len(s.history[key])-90:]
		}
	}
}

func (s *MemoryStore) Snapshot() []model.ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	statuses := make([]model.ServiceStatus, 0, len(s.latest))
	for key, latest := range s.latest {
		history := s.history[key]
		statuses = append(statuses, model.ServiceStatus{
			ProbeID:         latest.ProbeID,
			Name:            displayName(latest),
			Type:            latest.Type,
			CurrentStatus:   latest.Status,
			AvailabilityPct: availability(history),
			LastCheckedAt:   latest.CheckedAt,
			Timeline:        append([]model.StatusPoint(nil), history...),
			Message:         latest.Message,
		})
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})
	return statuses
}

func (s *MemoryStore) Nodes() map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]time.Time, len(s.nodes))
	for k, v := range s.nodes {
		out[k] = v
	}
	return out
}

func availability(points []model.StatusPoint) float64 {
	if len(points) == 0 {
		return 0
	}
	up := 0
	for _, point := range points {
		if point.Status == model.StatusUp {
			up++
		}
	}
	return float64(up) / float64(len(points)) * 100
}

func resultKey(nodeID, probeID string) string {
	return nodeID + "::" + probeID
}

func displayName(result model.CheckResult) string {
	if result.NodeName == "" {
		return result.Name
	}
	return result.NodeName + " / " + result.Name
}
