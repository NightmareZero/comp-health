package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Mode    string       `yaml:"mode"`
	Server  ServerConfig `yaml:"server"`
	Agent   AgentConfig  `yaml:"agent"`
	UI      UIConfig     `yaml:"ui"`
	Log     LogConfig    `yaml:"log"`
	Probes  []Probe      `yaml:"probes"`
	Storage Storage      `yaml:"storage"`
}

type ServerConfig struct {
	Listen           string        `yaml:"listen"`
	PublicURL        string        `yaml:"public_url"`
	Token            string        `yaml:"token"`
	EnableLocalProbe bool          `yaml:"enable_local_probe"`
	Retention        time.Duration `yaml:"retention"`
}

type AgentConfig struct {
	NodeID         string        `yaml:"node_id"`
	NodeName       string        `yaml:"node_name"`
	ServerURL      string        `yaml:"server_url"`
	Token          string        `yaml:"token"`
	ReportInterval time.Duration `yaml:"report_interval"`
	RetryInterval  time.Duration `yaml:"retry_interval"`
}

type UIConfig struct {
	Title string `yaml:"title"`
	Theme string `yaml:"theme"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

type Storage struct {
	Driver          string        `yaml:"driver"`
	Path            string        `yaml:"path"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	TimelinePoints  int           `yaml:"timeline_points"`
}

type Probe struct {
	ID           string            `yaml:"id"`
	Name         string            `yaml:"name"`
	Type         string            `yaml:"type"`
	Target       string            `yaml:"target"`
	Command      string            `yaml:"command"`
	Method       string            `yaml:"method"`
	ExpectStatus int               `yaml:"expect_status"`
	Timeout      time.Duration     `yaml:"timeout"`
	Interval     time.Duration     `yaml:"interval"`
	Headers      map[string]string `yaml:"headers"`
	Env          map[string]string `yaml:"env"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	applyDefaults(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func WriteExample(mode, out string) error {
	var content string
	if mode == "agent" {
		content = agentExample
	} else {
		content = serverExample
	}
	return os.WriteFile(out, []byte(content), 0o644)
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = ":8080"
	}
	if cfg.Server.Retention == 0 {
		cfg.Server.Retention = 30 * 24 * time.Hour
	}
	if cfg.Agent.ReportInterval == 0 {
		cfg.Agent.ReportInterval = 30 * time.Second
	}
	if cfg.Agent.RetryInterval == 0 {
		cfg.Agent.RetryInterval = 5 * time.Second
	}
	if cfg.UI.Title == "" {
		cfg.UI.Title = "Comp Health"
	}
	if cfg.UI.Theme == "" {
		cfg.UI.Theme = "auto"
	}
	if cfg.Storage.Driver == "" {
		cfg.Storage.Driver = "memory"
	}
	if cfg.Storage.CleanupInterval == 0 {
		cfg.Storage.CleanupInterval = time.Hour
	}
	if cfg.Storage.TimelinePoints == 0 {
		cfg.Storage.TimelinePoints = 96
	}
	if cfg.Storage.Path == "" && cfg.Storage.Driver == "sqlite" {
		cfg.Storage.Path = "data/health.db"
	}
	for i := range cfg.Probes {
		if cfg.Probes[i].Interval == 0 {
			cfg.Probes[i].Interval = 30 * time.Second
		}
		if cfg.Probes[i].Timeout == 0 {
			cfg.Probes[i].Timeout = 5 * time.Second
		}
		if cfg.Probes[i].Method == "" {
			cfg.Probes[i].Method = "GET"
		}
	}
}

func validate(cfg *Config) error {
	mode := strings.ToLower(cfg.Mode)
	if mode == "" {
		return errors.New("config mode is required")
	}
	if mode != "server" && mode != "agent" {
		return errors.New("config mode must be server or agent")
	}
	cfg.Mode = mode
	cfg.Storage.Driver = strings.ToLower(cfg.Storage.Driver)
	if cfg.Storage.Driver != "memory" && cfg.Storage.Driver != "sqlite" {
		return errors.New("storage driver must be memory or sqlite")
	}
	if cfg.Storage.Driver == "sqlite" && cfg.Storage.Path == "" {
		return errors.New("storage.path is required when storage driver is sqlite")
	}
	for _, probe := range cfg.Probes {
		if probe.ID == "" || probe.Name == "" || probe.Type == "" {
			return fmt.Errorf("probe id, name and type are required")
		}
	}
	return nil
}

const serverExample = `mode: server

server:
  listen: ":8080"
  public_url: "http://localhost:8080"
  token: "change-me"
  enable_local_probe: true
  retention: 720h

ui:
  title: "Comp Health"
  theme: "auto"

log:
  level: "info"

storage:
  driver: "sqlite"
  path: "data/health.db"
  cleanup_interval: 1h
  timeline_points: 96

probes:
  - id: "api-service"
    name: "API 服务"
    type: "http"
    target: "https://example.com/health"
    method: "GET"
    expect_status: 200
    interval: 30s
    timeout: 5s
`

const agentExample = `mode: agent

agent:
  node_id: "node-001"
  node_name: "edge-agent-01"
  server_url: "http://localhost:8080"
  token: "change-me"
  report_interval: 30s
  retry_interval: 5s

log:
  level: "info"

storage:
  driver: "memory"
  cleanup_interval: 1h
  timeline_points: 96

probes:
  - id: "api-service"
    name: "API 服务"
    type: "http"
    target: "https://example.com/health"
    method: "GET"
    expect_status: 200
    interval: 30s
    timeout: 5s

  - id: "shell-disk"
    name: "磁盘检查"
    type: "shell"
    command: "df -h / | tail -1"
    interval: 60s
    timeout: 5s
`
