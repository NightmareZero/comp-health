package model

import "time"

type Status string

const (
	StatusUp   Status = "up"
	StatusDown Status = "down"
)

type CheckResult struct {
	ProbeID    string            `json:"probe_id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Target     string            `json:"target,omitempty"`
	Status     Status            `json:"status"`
	LatencyMS  int64             `json:"latency_ms"`
	Message    string            `json:"message,omitempty"`
	CheckedAt  time.Time         `json:"checked_at"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	NodeID     string            `json:"node_id,omitempty"`
	NodeName   string            `json:"node_name,omitempty"`
}

type NodeReport struct {
	NodeID     string        `json:"node_id"`
	NodeName   string        `json:"node_name"`
	ReportedAt time.Time     `json:"reported_at"`
	Results    []CheckResult `json:"results"`
}

type ServiceStatus struct {
	ProbeID         string        `json:"probe_id"`
	Name            string        `json:"name"`
	Type            string        `json:"type"`
	CurrentStatus   Status        `json:"current_status"`
	AvailabilityPct float64       `json:"availability_pct"`
	LastCheckedAt   time.Time     `json:"last_checked_at"`
	Timeline        []StatusPoint `json:"timeline"`
	Message         string        `json:"message,omitempty"`
}

type StatusPoint struct {
	At        time.Time `json:"at"`
	Status    Status    `json:"status"`
	LatencyMS int64     `json:"latency_ms,omitempty"`
	Message   string    `json:"message,omitempty"`
	NodeID    string    `json:"node_id,omitempty"`
	NodeName  string    `json:"node_name,omitempty"`
}
