package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadConfig_ValidFile tests loading a valid configuration file
func TestLoadConfig_ValidFile(t *testing.T) {
	// Arrange
	content := `
server:
  metrics_port: 9090
  log_level: info
temperature:
  target_hdd: 38.0
  max_hdd: 45.0
  max_cpu: 75.0
  poll_interval: 60s
  warmest_disks: 4
fans:
  min_duty: 60
  max_duty: 100
  startup_duty: 50
pid:
  kp: 1.5
  ki: 0.05
  kd: 2.0
  integral_max: 20.0
disks:
  exclude_patterns:
    - "^loop"
    - "^sr"
`
	tmpFile := createTempConfig(t, content)
	defer os.Remove(tmpFile)

	// Act
	config, err := LoadConfig(tmpFile)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 9090, config.Server.MetricsPort)
	assert.Equal(t, "info", config.Server.LogLevel)
	assert.Equal(t, 38.0, config.Temperature.TargetHDD)
	assert.Equal(t, 45.0, config.Temperature.MaxHDD)
	assert.Equal(t, 75.0, config.Temperature.MaxCPU)
	assert.Equal(t, 60*time.Second, config.Temperature.PollInterval)
	assert.Equal(t, 4, config.Temperature.WarmestDisks)
	assert.Equal(t, 60, config.Fans.MinDuty)
	assert.Equal(t, 100, config.Fans.MaxDuty)
	assert.Equal(t, 50, config.Fans.StartupDuty)
	assert.Equal(t, 1.5, config.PID.Kp)
	assert.Equal(t, 0.05, config.PID.Ki)
	assert.Equal(t, 2.0, config.PID.Kd)
	assert.Equal(t, 20.0, config.PID.IntegralMax)
	assert.Len(t, config.Disks.ExcludePatterns, 2)
}

// TestLoadConfig_InvalidYAML tests loading a file with invalid YAML
func TestLoadConfig_InvalidYAML(t *testing.T) {
	// Arrange
	content := `
server:
  metrics_port: 9090
  invalid yaml here: [unclosed
`
	tmpFile := createTempConfig(t, content)
	defer os.Remove(tmpFile)

	// Act
	_, err := LoadConfig(tmpFile)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

// TestLoadConfig_MissingFile tests loading a non-existent file
func TestLoadConfig_MissingFile(t *testing.T) {
	// Act
	_, err := LoadConfig("/nonexistent/path/config.yaml")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

// TestLoadConfig_PartialConfig_UsesDefaults tests partial config with defaults
func TestLoadConfig_PartialConfig_UsesDefaults(t *testing.T) {
	// Arrange - only set a few values (use valid target < max)
	content := `
temperature:
  target_hdd: 37.0
  max_hdd: 42.0
fans:
  min_duty: 70
`
	tmpFile := createTempConfig(t, content)
	defer os.Remove(tmpFile)

	// Act
	config, err := LoadConfig(tmpFile)

	// Assert
	require.NoError(t, err)
	// Check custom values
	assert.Equal(t, 37.0, config.Temperature.TargetHDD)
	assert.Equal(t, 42.0, config.Temperature.MaxHDD)
	assert.Equal(t, 70, config.Fans.MinDuty)
	// Check defaults were applied
	assert.Equal(t, 9090, config.Server.MetricsPort)
	assert.Equal(t, "info", config.Server.LogLevel)
	assert.Equal(t, 75.0, config.Temperature.MaxCPU)
	assert.Equal(t, 30*time.Second, config.Temperature.PollInterval)
	assert.Equal(t, 4, config.Temperature.WarmestDisks)
	assert.Equal(t, 100, config.Fans.MaxDuty)
	assert.Equal(t, 50, config.Fans.StartupDuty)
	assert.Equal(t, 5.0, config.PID.Kp)
	assert.Equal(t, 0.1, config.PID.Ki)
	assert.Equal(t, 20.0, config.PID.Kd)
	assert.Equal(t, 50.0, config.PID.IntegralMax)
	assert.Len(t, config.Disks.ExcludePatterns, 5) // Default patterns
}

// TestSetDefaults_AllFieldsSet tests that defaults are applied to all fields
func TestSetDefaults_AllFieldsSet(t *testing.T) {
	// Arrange
	config := &Config{}

	// Act
	setDefaults(config)

	// Assert - verify all defaults are set
	assert.Equal(t, 9090, config.Server.MetricsPort)
	assert.Equal(t, "info", config.Server.LogLevel)
	assert.Equal(t, 38.0, config.Temperature.TargetHDD)
	assert.Equal(t, 40.0, config.Temperature.MaxHDD)
	assert.Equal(t, 75.0, config.Temperature.MaxCPU)
	assert.Equal(t, 30*time.Second, config.Temperature.PollInterval)
	assert.Equal(t, 4, config.Temperature.WarmestDisks)
	assert.Equal(t, 30, config.Fans.MinDuty)
	assert.Equal(t, 100, config.Fans.MaxDuty)
	assert.Equal(t, 50, config.Fans.StartupDuty)
	assert.Equal(t, 5.0, config.PID.Kp)
	assert.Equal(t, 0.1, config.PID.Ki)
	assert.Equal(t, 20.0, config.PID.Kd)
	assert.Equal(t, 50.0, config.PID.IntegralMax)
	assert.Len(t, config.Disks.ExcludePatterns, 5)
}

// TestSetDefaults_PartialInput tests defaults don't override existing values
func TestSetDefaults_PartialInput(t *testing.T) {
	// Arrange
	config := &Config{
		Server: ServerConfig{
			MetricsPort: 8080,
		},
		Temperature: TemperatureConfig{
			TargetHDD: 42.0,
		},
	}

	// Act
	setDefaults(config)

	// Assert - existing values preserved
	assert.Equal(t, 8080, config.Server.MetricsPort)
	assert.Equal(t, 42.0, config.Temperature.TargetHDD)
	// Assert - defaults applied to missing values
	assert.Equal(t, "info", config.Server.LogLevel)
	assert.Equal(t, 40.0, config.Temperature.MaxHDD)
}

// TestSetDefaults_EmptyConfig tests defaults on completely empty config
func TestSetDefaults_EmptyConfig(t *testing.T) {
	// Arrange
	config := &Config{}

	// Act
	setDefaults(config)

	// Assert - all defaults should be set
	assert.NotZero(t, config.Server.MetricsPort)
	assert.NotEmpty(t, config.Server.LogLevel)
	assert.NotZero(t, config.Temperature.TargetHDD)
	assert.NotZero(t, config.Fans.MinDuty)
	assert.NotZero(t, config.PID.Kp)
	assert.NotEmpty(t, config.Disks.ExcludePatterns)
}

// TestValidate_TargetHDD_GreaterThanMaxHDD_Error tests validation error
func TestValidate_TargetHDD_GreaterThanMaxHDD_Error(t *testing.T) {
	// Arrange
	config := &Config{
		Temperature: TemperatureConfig{
			TargetHDD:    45.0,
			MaxHDD:       40.0,
			MaxCPU:       75.0,
			PollInterval: 30 * time.Second,
			WarmestDisks: 4,
		},
	}
	setDefaults(config)

	// Act
	err := config.Validate()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target_hdd")
	assert.Contains(t, err.Error(), "must be less than")
}

// TestValidate_TargetHDD_Negative_Error tests negative target temp
func TestValidate_TargetHDD_Negative_Error(t *testing.T) {
	// Arrange
	config := &Config{
		Temperature: TemperatureConfig{
			TargetHDD:    -5.0,
			MaxHDD:       40.0,
			MaxCPU:       75.0,
			PollInterval: 30 * time.Second,
			WarmestDisks: 4,
		},
	}
	setDefaults(config)

	// Act
	err := config.Validate()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target_hdd must be positive")
}

// TestValidate_MaxHDD_Negative_Error tests negative max HDD temp
func TestValidate_MaxHDD_Negative_Error(t *testing.T) {
	// Arrange
	config := &Config{
		Temperature: TemperatureConfig{
			TargetHDD:    -50.0, // Also negative so target < max check passes
			MaxHDD:       -40.0,
			MaxCPU:       75.0,
			PollInterval: 30 * time.Second,
			WarmestDisks: 4,
		},
	}
	setDefaults(config)

	// Act
	err := config.Validate()

	// Assert
	require.Error(t, err)
	// Either error about target or max being negative is acceptable
	assert.True(t, strings.Contains(err.Error(), "must be positive") ||
		strings.Contains(err.Error(), "must be less than"))
}

// TestValidate_MaxCPU_Negative_Error tests negative max CPU temp
func TestValidate_MaxCPU_Negative_Error(t *testing.T) {
	// Arrange
	config := &Config{
		Temperature: TemperatureConfig{
			TargetHDD:    38.0,
			MaxHDD:       40.0,
			MaxCPU:       -75.0,
			PollInterval: 30 * time.Second,
			WarmestDisks: 4,
		},
	}
	setDefaults(config)

	// Act
	err := config.Validate()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_cpu must be positive")
}

// TestValidate_PollInterval_Zero_Error tests zero poll interval
func TestValidate_PollInterval_Zero_Error(t *testing.T) {
	// Arrange
	config := &Config{
		Temperature: TemperatureConfig{
			TargetHDD:    38.0,
			MaxHDD:       40.0,
			MaxCPU:       75.0,
			PollInterval: 0,
			WarmestDisks: 4,
		},
		Server: ServerConfig{
			MetricsPort: 9090,
			LogLevel:    "info",
		},
		Fans: FanConfig{
			MinDuty:     30,
			MaxDuty:     100,
			StartupDuty: 50,
		},
		PID: PIDConfig{
			Kp:          5.0,
			Ki:          0.1,
			Kd:          20.0,
			IntegralMax: 50.0,
		},
	}

	// Act
	err := config.Validate()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "poll_interval must be positive")
}

// TestValidate_WarmestDisks_Negative_Error tests negative warmest disks
func TestValidate_WarmestDisks_Negative_Error(t *testing.T) {
	// Arrange
	config := &Config{
		Temperature: TemperatureConfig{
			TargetHDD:    38.0,
			MaxHDD:       40.0,
			MaxCPU:       75.0,
			PollInterval: 30 * time.Second,
			WarmestDisks: -1,
		},
	}
	setDefaults(config)

	// Act
	err := config.Validate()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "warmest_disks must be positive")
}

// TestValidate_MinDuty_OutOfRange_Error tests min_duty out of range
func TestValidate_MinDuty_OutOfRange_Error(t *testing.T) {
	tests := []struct {
		name     string
		minDuty  int
		expected string
	}{
		{"negative", -10, "min_duty must be between 0-100"},
		{"above 100", 150, "min_duty must be between 0-100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			config := &Config{
				Fans: FanConfig{
					MinDuty:     tt.minDuty,
					MaxDuty:     100,
					StartupDuty: 50,
				},
			}
			setDefaults(config)

			// Act
			err := config.Validate()

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

// TestValidate_MaxDuty_OutOfRange_Error tests max_duty out of range
func TestValidate_MaxDuty_OutOfRange_Error(t *testing.T) {
	tests := []struct {
		name     string
		maxDuty  int
		expected string
	}{
		{"negative", -10, "max_duty must be between 0-100"},
		{"above 100", 150, "max_duty must be between 0-100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			config := &Config{
				Fans: FanConfig{
					MinDuty:     30,
					MaxDuty:     tt.maxDuty,
					StartupDuty: 50,
				},
			}
			setDefaults(config)

			// Act
			err := config.Validate()

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

// TestValidate_MinDuty_GreaterThanMaxDuty_Error tests min > max duty
func TestValidate_MinDuty_GreaterThanMaxDuty_Error(t *testing.T) {
	// Arrange
	config := &Config{
		Fans: FanConfig{
			MinDuty:     80,
			MaxDuty:     60,
			StartupDuty: 50,
		},
	}
	setDefaults(config)

	// Act
	err := config.Validate()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "min_duty")
	assert.Contains(t, err.Error(), "must be less than")
}

// TestValidate_StartupDuty_OutOfRange_Error tests startup_duty out of range
func TestValidate_StartupDuty_OutOfRange_Error(t *testing.T) {
	tests := []struct {
		name        string
		startupDuty int
		expected    string
	}{
		{"negative", -10, "startup_duty must be between 0-100"},
		{"above 100", 150, "startup_duty must be between 0-100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			config := &Config{
				Fans: FanConfig{
					MinDuty:     30,
					MaxDuty:     100,
					StartupDuty: tt.startupDuty,
				},
			}
			setDefaults(config)

			// Act
			err := config.Validate()

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

// TestValidate_NegativePID_Gains_Error tests negative PID gains
func TestValidate_NegativePID_Gains_Error(t *testing.T) {
	tests := []struct {
		name     string
		kp       float64
		ki       float64
		kd       float64
		expected string
	}{
		{"negative Kp", -1.0, 0.1, 2.0, "kp must be non-negative"},
		{"negative Ki", 1.5, -0.1, 2.0, "ki must be non-negative"},
		{"negative Kd", 1.5, 0.1, -2.0, "kd must be non-negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			config := &Config{
				PID: PIDConfig{
					Kp:          tt.kp,
					Ki:          tt.ki,
					Kd:          tt.kd,
					IntegralMax: 20.0,
				},
			}
			setDefaults(config)

			// Act
			err := config.Validate()

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

// TestValidate_ZeroIntegralMax_Error tests zero integral_max
func TestValidate_ZeroIntegralMax_Error(t *testing.T) {
	// Arrange
	config := &Config{
		Server: ServerConfig{
			MetricsPort: 9090,
			LogLevel:    "info",
		},
		Temperature: TemperatureConfig{
			TargetHDD:    38.0,
			MaxHDD:       40.0,
			MaxCPU:       75.0,
			PollInterval: 30 * time.Second,
			WarmestDisks: 4,
		},
		Fans: FanConfig{
			MinDuty:     30,
			MaxDuty:     100,
			StartupDuty: 50,
		},
		PID: PIDConfig{
			Kp:          1.5,
			Ki:          0.1,
			Kd:          2.0,
			IntegralMax: 0,
		},
	}

	// Act
	err := config.Validate()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "integral_max must be positive")
}

// TestValidate_InvalidPort_Error tests invalid metrics port
func TestValidate_InvalidPort_Error(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		expected string
	}{
		{"zero port", 0, "metrics_port must be between 1-65535"},
		{"negative port", -100, "metrics_port must be between 1-65535"},
		{"port too high", 70000, "metrics_port must be between 1-65535"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			config := &Config{
				Server: ServerConfig{
					MetricsPort: tt.port,
					LogLevel:    "info",
				},
				Temperature: TemperatureConfig{
					TargetHDD:    38.0,
					MaxHDD:       40.0,
					MaxCPU:       75.0,
					PollInterval: 30 * time.Second,
					WarmestDisks: 4,
				},
				Fans: FanConfig{
					MinDuty:     30,
					MaxDuty:     100,
					StartupDuty: 50,
				},
				PID: PIDConfig{
					Kp:          5.0,
					Ki:          0.1,
					Kd:          20.0,
					IntegralMax: 50.0,
				},
			}

			// Act
			err := config.Validate()

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

// TestValidate_InvalidLogLevel_Error tests invalid log level
func TestValidate_InvalidLogLevel_Error(t *testing.T) {
	// Arrange
	config := &Config{
		Server: ServerConfig{
			MetricsPort: 9090,
			LogLevel:    "invalid",
		},
	}
	setDefaults(config)

	// Act
	err := config.Validate()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log_level must be one of")
}

// TestValidate_AllFieldsValid tests that valid config passes validation
func TestValidate_AllFieldsValid(t *testing.T) {
	// Arrange
	config := &Config{
		Server: ServerConfig{
			MetricsPort: 9090,
			LogLevel:    "info",
		},
		Temperature: TemperatureConfig{
			TargetHDD:    38.0,
			MaxHDD:       45.0,
			MaxCPU:       75.0,
			PollInterval: 60 * time.Second,
			WarmestDisks: 4,
		},
		Fans: FanConfig{
			MinDuty:     60,
			MaxDuty:     100,
			StartupDuty: 50,
		},
		PID: PIDConfig{
			Kp:          1.5,
			Ki:          0.05,
			Kd:          2.0,
			IntegralMax: 20.0,
		},
		Disks: DiskConfig{
			ExcludePatterns: []string{"^loop", "^sr"},
		},
	}

	// Act
	err := config.Validate()

	// Assert
	assert.NoError(t, err)
}

// TestConfig_RoundTrip_YAML tests config can be marshaled and unmarshaled
func TestConfig_RoundTrip_YAML(t *testing.T) {
	// Arrange
	content := `
server:
  metrics_port: 9090
  log_level: debug
temperature:
  target_hdd: 42.0
  max_hdd: 50.0
  max_cpu: 80.0
  poll_interval: 120s
  warmest_disks: 6
fans:
  min_duty: 70
  max_duty: 100
  startup_duty: 80
pid:
  kp: 3.0
  ki: 0.2
  kd: 5.0
  integral_max: 30.0
disks:
  exclude_patterns:
    - "^loop"
    - "^zd"
    - "^dm-"
`
	tmpFile := createTempConfig(t, content)
	defer os.Remove(tmpFile)

	// Act - load config
	config, err := LoadConfig(tmpFile)
	require.NoError(t, err)

	// Assert - verify all values match
	assert.Equal(t, 9090, config.Server.MetricsPort)
	assert.Equal(t, "debug", config.Server.LogLevel)
	assert.Equal(t, 42.0, config.Temperature.TargetHDD)
	assert.Equal(t, 50.0, config.Temperature.MaxHDD)
	assert.Equal(t, 80.0, config.Temperature.MaxCPU)
	assert.Equal(t, 120*time.Second, config.Temperature.PollInterval)
	assert.Equal(t, 6, config.Temperature.WarmestDisks)
	assert.Equal(t, 70, config.Fans.MinDuty)
	assert.Equal(t, 100, config.Fans.MaxDuty)
	assert.Equal(t, 80, config.Fans.StartupDuty)
	assert.Equal(t, 3.0, config.PID.Kp)
	assert.Equal(t, 0.2, config.PID.Ki)
	assert.Equal(t, 5.0, config.PID.Kd)
	assert.Equal(t, 30.0, config.PID.IntegralMax)
	assert.Equal(t, []string{"^loop", "^zd", "^dm-"}, config.Disks.ExcludePatterns)
}

// Helper function to create a temporary config file for testing
func createTempConfig(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(tmpFile, []byte(strings.TrimSpace(content)), 0644)
	require.NoError(t, err)
	return tmpFile
}
