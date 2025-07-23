package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type ServerConfig struct {
	Port         int           `json:"port"`
	Host         string        `json:"host"`
	ReadTimeout  Duration      `json:"read_timeout"`
	WriteTimeout Duration      `json:"write_timeout"`
	IdleTimeout  Duration      `json:"idle_timeout"`
}

type AuthConfig struct {
	APIKeys     []string `json:"api_keys"`
	RequireAuth bool     `json:"require_auth"`
}

type CLIConfig struct {
	ProjectPath   string            `json:"project_path"`
	WorkingDir    string            `json:"working_dir"`
	APIKeys       map[string]string `json:"api_keys,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	AutoDetectSTL bool              `json:"auto_detect_stl"`
}

type WebhookConfig struct {
	Enabled      bool          `json:"enabled"`
	MaxRetries   int           `json:"max_retries"`
	RetryBackoff time.Duration `json:"retry_backoff"`
	Secret       string        `json:"secret"`
}

type JobsConfig struct {
	MaxConcurrent   int           `json:"max_concurrent"`
	CleanupAfter    time.Duration `json:"cleanup_after"`
	DefaultTTL      time.Duration `json:"default_ttl"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	MaxHistorySize  int           `json:"max_history_size"`
}

type SecurityConfig struct {
	EnableCORS     bool     `json:"enable_cors"`
	AllowedOrigins []string `json:"allowed_origins"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

type Config struct {
	Server   ServerConfig    `json:"server"`
	Auth     AuthConfig      `json:"auth"`
	CLI      CLIConfig       `json:"cli"`
	Webhooks WebhookConfig   `json:"webhooks"`
	Jobs     JobsConfig      `json:"jobs"`
	Security SecurityConfig  `json:"security"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %v", err)
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config: %v", err)
	}

	// Set defaults
	if cfg.Server.Host == "" {
		cfg.Server.Host = "localhost"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout.Duration == 0 {
		cfg.Server.ReadTimeout.Duration = 30 * time.Second
	}
	if cfg.Server.WriteTimeout.Duration == 0 {
		cfg.Server.WriteTimeout.Duration = 30 * time.Second
	}
	if cfg.Server.IdleTimeout.Duration == 0 {
		cfg.Server.IdleTimeout.Duration = 60 * time.Second
	}
	if cfg.Jobs.MaxConcurrent == 0 {
		cfg.Jobs.MaxConcurrent = 5
	}
	if cfg.Jobs.CleanupAfter == 0 {
		cfg.Jobs.CleanupAfter = 24 * time.Hour
	}
	if cfg.Jobs.DefaultTTL == 0 {
		cfg.Jobs.DefaultTTL = 1 * time.Hour
	}
	if cfg.Jobs.CleanupInterval == 0 {
		cfg.Jobs.CleanupInterval = 10 * time.Minute
	}
	if cfg.Jobs.MaxHistorySize == 0 {
		cfg.Jobs.MaxHistorySize = 1000
	}
	if cfg.Webhooks.MaxRetries == 0 {
		cfg.Webhooks.MaxRetries = 3
	}
	if cfg.Webhooks.RetryBackoff == 0 {
		cfg.Webhooks.RetryBackoff = 5 * time.Second
	}
	if cfg.CLI.AutoDetectSTL {
		cfg.CLI.WorkingDir = findSTLDirectory()
	}
	if cfg.CLI.WorkingDir == "" {
		cfg.CLI.WorkingDir = "."
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Server.Port)
	}
	return nil
}

// findSTLDirectory searches for STL project directory
func findSTLDirectory() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	
	stlPath := homeDir + "/STL"
	if _, err := os.Stat(stlPath + "/.plandex-v2"); err == nil {
		return stlPath
	}
	
	return "."
}
