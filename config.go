package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration structure
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Temperature TemperatureConfig `yaml:"temperature"`
	Fans        FanConfig         `yaml:"fans"`
	PID         PIDConfig         `yaml:"pid"`
	Disks       DiskConfig        `yaml:"disks"`
}

// ServerConfig contains server-related settings
type ServerConfig struct {
	MetricsPort int    `yaml:"metrics_port"`
	LogLevel    string `yaml:"log_level"`
}

// TemperatureConfig contains temperature thresholds and polling settings
type TemperatureConfig struct {
	TargetHDD      float64       `yaml:"target_hdd"`      // Target temp for warmest N disks (°C)
	MaxHDD         float64       `yaml:"max_hdd"`          // Emergency override temp (°C)
	MaxCPU         float64       `yaml:"max_cpu"`          // CPU emergency temp (°C)
	PollInterval   time.Duration `yaml:"poll_interval"`    // How often to check temps and adjust fans
	WarmestDisks   int           `yaml:"warmest_disks"`    // Average temp of this many warmest disks
}

// FanConfig contains fan control settings
type FanConfig struct {
	MinDuty     int `yaml:"min_duty"`     // Minimum fan duty cycle (%)
	MaxDuty     int `yaml:"max_duty"`     // Maximum fan duty cycle (%)
	StartupDuty int `yaml:"startup_duty"` // Initial fan duty on startup (%)
}

// PIDConfig contains PID controller gains and limits
type PIDConfig struct {
	Kp          float64 `yaml:"kp"`           // Proportional gain
	Ki          float64 `yaml:"ki"`           // Integral gain
	Kd          float64 `yaml:"kd"`           // Derivative gain
	IntegralMax float64 `yaml:"integral_max"` // Anti-windup limit for integral term
}

// DiskConfig contains disk discovery and filtering settings
type DiskConfig struct {
	ExcludePatterns []string `yaml:"exclude_patterns"` // Regex patterns for disks to ignore
}

// LoadConfig loads and parses the configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Set defaults for any missing values
	setDefaults(&config)

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults sets default values for any missing configuration fields
func setDefaults(config *Config) {
	if config.Server.MetricsPort == 0 {
		config.Server.MetricsPort = 9090
	}
	if config.Server.LogLevel == "" {
		config.Server.LogLevel = "info"
	}
	if config.Temperature.TargetHDD == 0 {
		config.Temperature.TargetHDD = 38.0
	}
	if config.Temperature.MaxHDD == 0 {
		config.Temperature.MaxHDD = 40.0
	}
	if config.Temperature.MaxCPU == 0 {
		config.Temperature.MaxCPU = 75.0
	}
	if config.Temperature.PollInterval == 0 {
		config.Temperature.PollInterval = 30 * time.Second
	}
	if config.Temperature.WarmestDisks == 0 {
		config.Temperature.WarmestDisks = 4
	}
	if config.Fans.MinDuty == 0 {
		config.Fans.MinDuty = 30
	}
	if config.Fans.MaxDuty == 0 {
		config.Fans.MaxDuty = 100
	}
	if config.Fans.StartupDuty == 0 {
		config.Fans.StartupDuty = 50
	}
	if config.PID.Kp == 0 {
		config.PID.Kp = 5.0
	}
	if config.PID.Ki == 0 {
		config.PID.Ki = 0.1
	}
	if config.PID.Kd == 0 {
		config.PID.Kd = 20.0
	}
	if config.PID.IntegralMax == 0 {
		config.PID.IntegralMax = 50.0
	}
	if len(config.Disks.ExcludePatterns) == 0 {
		config.Disks.ExcludePatterns = []string{
			"^loop",
			"^sr",
			"^zram",
			"^zd",
			"^dm-",
		}
	}
}

// Validate checks all configuration values for logical consistency
func (c *Config) Validate() error {
	// Temperature validation
	if c.Temperature.TargetHDD >= c.Temperature.MaxHDD {
		return fmt.Errorf("target_hdd (%.1f) must be less than max_hdd (%.1f)", 
			c.Temperature.TargetHDD, c.Temperature.MaxHDD)
	}
	if c.Temperature.TargetHDD <= 0 {
		return fmt.Errorf("target_hdd must be positive, got %.1f", c.Temperature.TargetHDD)
	}
	if c.Temperature.MaxHDD <= 0 {
		return fmt.Errorf("max_hdd must be positive, got %.1f", c.Temperature.MaxHDD)
	}
	if c.Temperature.MaxCPU <= 0 {
		return fmt.Errorf("max_cpu must be positive, got %.1f", c.Temperature.MaxCPU)
	}
	if c.Temperature.PollInterval <= 0 {
		return fmt.Errorf("poll_interval must be positive, got %v", c.Temperature.PollInterval)
	}
	if c.Temperature.WarmestDisks <= 0 {
		return fmt.Errorf("warmest_disks must be positive, got %d", c.Temperature.WarmestDisks)
	}

	// Fan validation
	if c.Fans.MinDuty < 0 || c.Fans.MinDuty > 100 {
		return fmt.Errorf("min_duty must be between 0-100, got %d", c.Fans.MinDuty)
	}
	if c.Fans.MaxDuty < 0 || c.Fans.MaxDuty > 100 {
		return fmt.Errorf("max_duty must be between 0-100, got %d", c.Fans.MaxDuty)
	}
	if c.Fans.StartupDuty < 0 || c.Fans.StartupDuty > 100 {
		return fmt.Errorf("startup_duty must be between 0-100, got %d", c.Fans.StartupDuty)
	}
	if c.Fans.MinDuty >= c.Fans.MaxDuty {
		return fmt.Errorf("min_duty (%d) must be less than max_duty (%d)", 
			c.Fans.MinDuty, c.Fans.MaxDuty)
	}

	// PID validation
	if c.PID.Kp < 0 {
		return fmt.Errorf("kp must be non-negative, got %.3f", c.PID.Kp)
	}
	if c.PID.Ki < 0 {
		return fmt.Errorf("ki must be non-negative, got %.3f", c.PID.Ki)
	}
	if c.PID.Kd < 0 {
		return fmt.Errorf("kd must be non-negative, got %.3f", c.PID.Kd)
	}
	if c.PID.IntegralMax <= 0 {
		return fmt.Errorf("integral_max must be positive, got %.3f", c.PID.IntegralMax)
	}

	// Server validation
	if c.Server.MetricsPort <= 0 || c.Server.MetricsPort > 65535 {
		return fmt.Errorf("metrics_port must be between 1-65535, got %d", c.Server.MetricsPort)
	}
	if c.Server.LogLevel != "debug" && c.Server.LogLevel != "info" && 
	   c.Server.LogLevel != "warn" && c.Server.LogLevel != "error" {
		return fmt.Errorf("log_level must be one of: debug, info, warn, error, got %s", c.Server.LogLevel)
	}

	return nil
}
