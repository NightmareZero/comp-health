package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"comp-health/internal/model"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db        *sql.DB
	retention time.Duration
}

func NewSQLiteStore(ctx context.Context, path string, retention, cleanupInterval time.Duration) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	store := &SQLiteStore{db: db, retention: retention}
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	store.applyPragmas()
	if cleanupInterval > 0 && retention > 0 {
		go store.startCleanup(ctx, cleanupInterval)
	}
	return store, nil
}

func (s *SQLiteStore) SaveResult(result model.CheckResult) {
	if result.CheckedAt.IsZero() {
		result.CheckedAt = time.Now()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	if err := s.insertResult(tx, result); err != nil {
		return
	}
	if err := s.upsertLatest(tx, result); err != nil {
		return
	}
	_ = tx.Commit()
}

func (s *SQLiteStore) SaveReport(report model.NodeReport) {
	if len(report.Results) == 0 {
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	for _, result := range report.Results {
		result.NodeID = report.NodeID
		result.NodeName = report.NodeName
		if err := s.insertResult(tx, result); err != nil {
			return
		}
		if err := s.upsertLatest(tx, result); err != nil {
			return
		}
	}
	_ = tx.Commit()
}

func (s *SQLiteStore) Snapshot(rangeWindow time.Duration, points int) []model.ServiceStatus {
	latestRows, err := s.db.Query(`
		SELECT probe_id, name, type, status, latency_ms, message, checked_at, node_id, node_name
		FROM latest_status
		ORDER BY display_name ASC
	`)
	if err != nil {
		return nil
	}
	defer latestRows.Close()

	statuses := make([]model.ServiceStatus, 0)
	for latestRows.Next() {
		var (
			probeID   string
			name      string
			typ       string
			status    string
			latencyMS int64
			message   string
			checkedAt time.Time
			nodeID    string
			nodeName  string
		)
		if err := latestRows.Scan(&probeID, &name, &typ, &status, &latencyMS, &message, &checkedAt, &nodeID, &nodeName); err != nil {
			continue
		}
		result := model.CheckResult{
			ProbeID:   probeID,
			Name:      name,
			Type:      typ,
			Status:    model.Status(status),
			LatencyMS: latencyMS,
			Message:   message,
			CheckedAt: checkedAt,
			NodeID:    nodeID,
			NodeName:  nodeName,
		}
		timeline := s.timelineFor(result, rangeWindow, points)
		statuses = append(statuses, model.ServiceStatus{
			ProbeID:         result.ProbeID,
			Name:            displayName(result),
			Type:            result.Type,
			CurrentStatus:   result.Status,
			AvailabilityPct: availability(timeline),
			LastCheckedAt:   result.CheckedAt,
			Timeline:        timeline,
			Message:         result.Message,
		})
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})
	return statuses
}

func (s *SQLiteStore) Nodes() map[string]time.Time {
	rows, err := s.db.Query(`
		SELECT node_id, MAX(checked_at) AS last_seen
		FROM latest_status
		WHERE node_id <> ''
		GROUP BY node_id
	`)
	if err != nil {
		return map[string]time.Time{}
	}
	defer rows.Close()
	out := make(map[string]time.Time)
	for rows.Next() {
		var nodeID string
		var lastSeen time.Time
		if err := rows.Scan(&nodeID, &lastSeen); err != nil {
			continue
		}
		out[nodeID] = lastSeen
	}
	return out
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) initSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS check_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			probe_id TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			message TEXT NOT NULL DEFAULT '',
			checked_at TIMESTAMP NOT NULL,
			node_id TEXT NOT NULL DEFAULT '',
			node_name TEXT NOT NULL DEFAULT '',
			display_name TEXT NOT NULL,
			metadata_json TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_check_results_scope_time
		ON check_results (node_id, probe_id, checked_at);
		CREATE INDEX IF NOT EXISTS idx_check_results_checked_at
		ON check_results (checked_at);

		CREATE TABLE IF NOT EXISTS latest_status (
			probe_id TEXT NOT NULL,
			node_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			message TEXT NOT NULL DEFAULT '',
			checked_at TIMESTAMP NOT NULL,
			node_name TEXT NOT NULL DEFAULT '',
			display_name TEXT NOT NULL,
			PRIMARY KEY (node_id, probe_id)
		);
	`) 
	if err != nil {
		return fmt.Errorf("init sqlite schema: %w", err)
	}
	return nil
}

func (s *SQLiteStore) applyPragmas() {
	_, _ = s.db.Exec(`PRAGMA journal_mode = WAL;`)
	_, _ = s.db.Exec(`PRAGMA synchronous = NORMAL;`)
	_, _ = s.db.Exec(`PRAGMA busy_timeout = 5000;`)
}

func (s *SQLiteStore) startCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.cleanup()
		}
	}
}

func (s *SQLiteStore) cleanup() error {
	if s.retention <= 0 {
		return nil
	}
	cutoff := time.Now().Add(-s.retention)
	_, err := s.db.Exec(`DELETE FROM check_results WHERE checked_at < ?`, cutoff)
	return err
}

func (s *SQLiteStore) insertResult(tx *sql.Tx, result model.CheckResult) error {
	metadataJSON := ""
	_, err := tx.Exec(`
		INSERT INTO check_results (
			probe_id, name, type, status, latency_ms, message, checked_at, node_id, node_name, display_name, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, result.ProbeID, result.Name, result.Type, string(result.Status), result.LatencyMS, result.Message, result.CheckedAt, result.NodeID, result.NodeName, displayName(result), metadataJSON)
	return err
}

func (s *SQLiteStore) upsertLatest(tx *sql.Tx, result model.CheckResult) error {
	_, err := tx.Exec(`
		INSERT INTO latest_status (
			probe_id, node_id, name, type, status, latency_ms, message, checked_at, node_name, display_name
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id, probe_id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			status = excluded.status,
			latency_ms = excluded.latency_ms,
			message = excluded.message,
			checked_at = excluded.checked_at,
			node_name = excluded.node_name,
			display_name = excluded.display_name
	`, result.ProbeID, result.NodeID, result.Name, result.Type, string(result.Status), result.LatencyMS, result.Message, result.CheckedAt, result.NodeName, displayName(result))
	return err
}

func (s *SQLiteStore) timelineFor(result model.CheckResult, rangeWindow time.Duration, points int) []model.StatusPoint {
	if points <= 0 {
		points = 96
	}
	query := `
		SELECT checked_at, status, latency_ms, message, node_id, node_name
		FROM check_results
		WHERE probe_id = ? AND node_id = ?
	`
	args := []any{result.ProbeID, result.NodeID}
	if rangeWindow > 0 {
		query += ` AND checked_at >= ?`
		args = append(args, time.Now().Add(-rangeWindow))
	}
	query += ` ORDER BY checked_at ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return []model.StatusPoint{}
	}
	defer rows.Close()
	items := make([]model.StatusPoint, 0)
	for rows.Next() {
		var point model.StatusPoint
		var status string
		if err := rows.Scan(&point.At, &status, &point.LatencyMS, &point.Message, &point.NodeID, &point.NodeName); err != nil {
			continue
		}
		point.Status = model.Status(status)
		items = append(items, point)
	}
	return aggregateTimeline(items, rangeWindow, points)
}

var errStoreClosed = errors.New("store closed")
