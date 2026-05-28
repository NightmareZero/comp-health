package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"comp-health/internal/config"
	"comp-health/internal/model"
	"comp-health/internal/store"
)

// Server is the HTTP server that serves the Web UI and API endpoints.
type Server struct {
	cfg     *config.Config
	storage store.Store
	webFS   fs.FS
}

// New creates a new Server with the given configuration, store, and embedded web FS.
func New(cfg *config.Config, storage store.Store, webFS fs.FS) *Server {
	return &Server{cfg: cfg, storage: storage, webFS: webFS}
}

// Run starts the HTTP server and blocks until ctx is cancelled or a fatal error occurs.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("POST /api/v1/reports", s.bearerAuth(s.handleReport))
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/nodes", s.handleNodes)
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	// Embedded Web UI
	sub, err := fs.Sub(s.webFS, "web")
	if err != nil {
		return fmt.Errorf("sub web fs: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	srv := &http.Server{
		Addr:         s.cfg.Server.Listen,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

// statusResponse is the payload returned by GET /api/v1/status.
type statusResponse struct {
	Overall   string                `json:"overall"`
	UpdatedAt time.Time             `json:"updated_at"`
	Range     string                `json:"range"`
	Services  []model.ServiceStatus `json:"services"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	rangeKey := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("range")))
	rangeWindow := parseRangeWindow(rangeKey)
	if rangeKey == "" {
		rangeKey = "6h"
	}
	services := s.storage.Snapshot(rangeWindow, s.cfg.Storage.TimelinePoints)
	overall := "up"
	updatedAt := time.Time{}
	for _, svc := range services {
		if svc.CurrentStatus == model.StatusDown {
			overall = "down"
		}
		if svc.LastCheckedAt.After(updatedAt) {
			updatedAt = svc.LastCheckedAt
		}
	}
	writeJSON(w, http.StatusOK, statusResponse{
		Overall:   overall,
		UpdatedAt: updatedAt,
		Range:     rangeKey,
		Services:  services,
	})
}

// nodeInfo is the payload item returned by GET /api/v1/nodes.
type nodeInfo struct {
	NodeID   string    `json:"node_id"`
	LastSeen time.Time `json:"last_seen"`
	Online   bool      `json:"online"`
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	nodes := s.storage.Nodes()
	threshold := 2 * s.cfg.Agent.ReportInterval
	if threshold <= 0 {
		threshold = 2 * time.Minute
	}
	now := time.Now()
	result := make([]nodeInfo, 0, len(nodes))
	for id, lastSeen := range nodes {
		result = append(result, nodeInfo{
			NodeID:   id,
			LastSeen: lastSeen,
			Online:   now.Sub(lastSeen) < threshold,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	var report model.NodeReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if report.ReportedAt.IsZero() {
		report.ReportedAt = time.Now()
	}
	for i := range report.Results {
		if report.Results[i].CheckedAt.IsZero() {
			report.Results[i].CheckedAt = report.ReportedAt
		}
	}
	s.storage.SaveReport(report)
	log.Printf("received report from node=%s probes=%d", report.NodeID, len(report.Results))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// bearerAuth wraps a handler with Bearer token authentication.
// If the server token is empty, authentication is skipped.
func (s *Server) bearerAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Server.Token == "" {
			next(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" || token != s.cfg.Server.Token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func parseRangeWindow(value string) time.Duration {
	switch value {
	case "12h":
		return 12 * time.Hour
	case "3d":
		return 72 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "6h", "":
		return 6 * time.Hour
	default:
		return 6 * time.Hour
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json error: %v", err)
	}
}
