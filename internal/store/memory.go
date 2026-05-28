package store

import (
	"sort"
	"sync"
	"time"

	"comp-health/internal/model"
)

type MemoryStore struct {
	mu        sync.RWMutex
	latest    map[string]model.CheckResult
	latestIDs []string
	history   map[string][]model.StatusPoint
	nodes     map[string]time.Time
	nodeName  map[string]string
	retention time.Duration
}

func NewMemoryStore(retention time.Duration) *MemoryStore {
	if retention <= 0 {
		retention = 30 * 24 * time.Hour
	}
	return &MemoryStore{
		latest:    make(map[string]model.CheckResult),
		history:   make(map[string][]model.StatusPoint),
		nodes:     make(map[string]time.Time),
		nodeName:  make(map[string]string),
		retention: retention,
	}
}

func (s *MemoryStore) SaveResult(result model.CheckResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveResultLocked(result)
}

func (s *MemoryStore) SaveReport(report model.NodeReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[report.NodeID] = report.ReportedAt
	s.nodeName[report.NodeID] = report.NodeName
	for _, result := range report.Results {
		result.NodeID = report.NodeID
		result.NodeName = report.NodeName
		s.saveResultLocked(result)
	}
}

func (s *MemoryStore) Snapshot(rangeWindow time.Duration, points int) []model.ServiceStatus {
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
			AvailabilityPct: availability(selectWindow(history, rangeWindow)),
			LastCheckedAt:   latest.CheckedAt,
			Timeline:        aggregateTimeline(history, rangeWindow, points),
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

func (s *MemoryStore) Close() error {
	return nil
}

func (s *MemoryStore) saveResultLocked(result model.CheckResult) {
	key := resultKey(result.NodeID, result.ProbeID)
	s.latest[key] = result
	s.history[key] = append(s.history[key], statusPointFromResult(result))
	s.history[key] = trimPointsByRetention(s.history[key], s.retention)
	if result.NodeID != "" {
		s.nodes[result.NodeID] = result.CheckedAt
		s.nodeName[result.NodeID] = result.NodeName
	}
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

func statusPointFromResult(result model.CheckResult) model.StatusPoint {
	return model.StatusPoint{
		At:        result.CheckedAt,
		Status:    result.Status,
		LatencyMS: result.LatencyMS,
		Message:   result.Message,
		NodeID:    result.NodeID,
		NodeName:  result.NodeName,
	}
}

func trimPointsByRetention(points []model.StatusPoint, retention time.Duration) []model.StatusPoint {
	if len(points) == 0 || retention <= 0 {
		return points
	}
	cutoff := time.Now().Add(-retention)
	idx := 0
	for idx < len(points) && points[idx].At.Before(cutoff) {
		idx++
	}
	if idx == 0 {
		return points
	}
	trimmed := append([]model.StatusPoint(nil), points[idx:]...)
	if len(trimmed) == 0 {
		return []model.StatusPoint{}
	}
	return trimmed
}

func selectWindow(points []model.StatusPoint, rangeWindow time.Duration) []model.StatusPoint {
	if len(points) == 0 || rangeWindow <= 0 {
		return append([]model.StatusPoint(nil), points...)
	}
	cutoff := time.Now().Add(-rangeWindow)
	idx := 0
	for idx < len(points) && points[idx].At.Before(cutoff) {
		idx++
	}
	return append([]model.StatusPoint(nil), points[idx:]...)
}

func aggregateTimeline(points []model.StatusPoint, rangeWindow time.Duration, maxPoints int) []model.StatusPoint {
	windowPoints := selectWindow(points, rangeWindow)
	if len(windowPoints) == 0 {
		return []model.StatusPoint{}
	}
	if maxPoints <= 0 || len(windowPoints) <= maxPoints {
		return append([]model.StatusPoint(nil), windowPoints...)
	}
	bucketSize := (len(windowPoints) + maxPoints - 1) / maxPoints
	out := make([]model.StatusPoint, 0, maxPoints)
	for i := 0; i < len(windowPoints); i += bucketSize {
		end := i + bucketSize
		if end > len(windowPoints) {
			end = len(windowPoints)
		}
		out = append(out, summarizeBucket(windowPoints[i:end]))
	}
	return out
}

func summarizeBucket(points []model.StatusPoint) model.StatusPoint {
	best := points[len(points)-1]
	for _, point := range points {
		if point.Status == model.StatusDown {
			best = point
		}
		if point.At.After(best.At) {
			best.At = point.At
		}
		if point.LatencyMS > best.LatencyMS {
			best.LatencyMS = point.LatencyMS
		}
		if best.Message == "" && point.Message != "" {
			best.Message = point.Message
		}
		if best.NodeID == "" && point.NodeID != "" {
			best.NodeID = point.NodeID
			best.NodeName = point.NodeName
		}
	}
	return best
}
