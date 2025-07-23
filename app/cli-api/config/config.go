package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Duration is a custom type that can be unmarshaled from JSON strings
type Duration struct {
	time.Duration
}

// UnmarshalJSON implements json.Unmarshaler
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = duration
	return nil
}

// MarshalJSON implements json.Marshaler
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

// Config represents the API wrapper configuration
type Config struct {
	Server   ServerConfig   `json:"server"`
	Auth     AuthConfig     `json:"auth"`
	CLI      CLIConfig      `json:"cli"`
	Jobs     JobsConfig     `json:"jobs"`
	Webhooks WebhooksConfig `json:"webhooks"`
	Security SecurityConfig `json:"security"`
}

// ServerConfig configures the HTTP server
type ServerConfig struct {
	Port         int      `json:"port"`
	Host         string   `json:"host"`
	ReadTimeout  Duration `json:"read_timeout"`
	WriteTimeout Duration `json:"write_timeout"`
	IdleTimeout  Duration `json:"idle_timeout"`
}

// AuthConfig configures API authentication
type AuthConfig struct {
	APIKeys       []string `json:"api_keys"`
	RequireAuth   bool     `json:"require_auth"`
	TokenLifetime string   `json:"token_lifetime"`
}

// CLIConfig configures CLI integration
type CLIConfig struct {
	ProjectPath   string            `json:"project_path"`
	PlandexBinary string            `json:"plandex_binary"`
	WorkingDir    string            `json:"working_dir"`
	Environment   map[string]string `json:"environment"`
	Timeout       Duration          `json:"timeout"`
}

// JobsConfig configures job management
type JobsConfig struct {
	MaxConcurrent   int      `json:"max_concurrent"`
	DefaultTTL      Duration `json:"default_ttl"`
	CleanupInterval Duration `json:"cleanup_interval"`
	MaxHistorySize  int      `json:"max_history_size"`
}

// WebhooksConfig configures webhook support
type WebhooksConfig struct {
	Enabled      bool     `json:"enabled"`
	Secret       string   `json:"secret"`
	MaxRetries   int      `json:"max_retries"`
	RetryBackoff Duration `json:"retry_backoff"`
}

// SecurityConfig configures security settings
type SecurityConfig struct {
	EnableCORS     bool     `json:"enable_cors"`
	AllowedOrigins []string `json:"allowed_origins"`
	RateLimit      int      `json:"rate_limit"`
	TrustedProxies []string `json:"trusted_proxies"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         8080,
			Host:         "localhost",
			ReadTimeout:  Duration{30 * time.Second},
			WriteTimeout: Duration{30 * time.Second},
			IdleTimeout:  Duration{60 * time.Second},
		},
		Auth: AuthConfig{
			APIKeys:       []string{},
			RequireAuth:   true,
			TokenLifetime: "24h",
		},
		CLI: CLIConfig{
			ProjectPath:   ".",
			PlandexBinary: "plandex",
			WorkingDir:    ".",
			Environment:   make(map[string]string),
			Timeout:       Duration{10 * time.Minute},
		},
		Jobs: JobsConfig{
			MaxConcurrent:   5,
			DefaultTTL:      Duration{24 * time.Hour},
			CleanupInterval: Duration{1 * time.Hour},
			MaxHistorySize:  1000,
		},
		Webhooks: WebhooksConfig{
			Enabled:      false,
			Secret:       "",
			MaxRetries:   3,
			RetryBackoff: Duration{30 * time.Second},
		},
		Security: SecurityConfig{
			EnableCORS:     true,
			AllowedOrigins: []string{"*"},
			RateLimit:      100, // requests per minute
			TrustedProxies: []string{},
		},
	}
}

// Load loads configuration from file or creates default
func Load(configPath string) (*Config, error) {
	// If no config path specified, try to find default
	if configPath == "" {
		configPath = findDefaultConfigPath()
	}

	// If config file doesn't exist, create default
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := DefaultConfig()
		return cfg, SaveConfig(cfg, configPath)
	}

	// Load existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// SaveConfig saves configuration to file
func SaveConfig(cfg *Config, configPath string) error {
	// Create directory if needed
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// findDefaultConfigPath finds the default configuration file path
func findDefaultConfigPath() string {
	// Check current directory first
	if _, err := os.Stat("plandex-api.json"); err == nil {
		return "plandex-api.json"
	}

	// Check .plandex-v2 directory if it exists
	if _, err := os.Stat(".plandex-v2"); err == nil {
		return filepath.Join(".plandex-v2", "api-config.json")
	}

	// Default to current directory
	return "plandex-api.json"
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Auth.RequireAuth && len(c.Auth.APIKeys) == 0 {
		return fmt.Errorf("authentication required but no API keys configured")
	}

	if c.CLI.ProjectPath == "" {
		return fmt.Errorf("CLI project path cannot be empty")
	}

	if c.Jobs.MaxConcurrent < 1 {
		return fmt.Errorf("max concurrent jobs must be at least 1")
	}

	return nil
}
