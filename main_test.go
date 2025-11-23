package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCheckEmergencyConditions_CPUOverTemp tests CPU emergency condition
func TestCheckEmergencyConditions_CPUOverTemp(t *testing.T) {
	// Arrange
	config := &Config{
		Temperature: TemperatureConfig{
			MaxCPU: 75.0,
			MaxHDD: 45.0,
		},
	}
	cpuTemp := 80.0   // Above max
	maxDiskTemp := 40 // Normal

	// Act
	reason := checkEmergencyConditions(cpuTemp, maxDiskTemp, config)

	// Assert
	assert.Equal(t, "cpu_temp", reason)
}

// TestCheckEmergencyConditions_HDDOverTemp tests HDD emergency condition
func TestCheckEmergencyConditions_HDDOverTemp(t *testing.T) {
	// Arrange
	config := &Config{
		Temperature: TemperatureConfig{
			MaxCPU: 75.0,
			MaxHDD: 45.0,
		},
	}
	cpuTemp := 60.0   // Normal
	maxDiskTemp := 50 // Above max

	// Act
	reason := checkEmergencyConditions(cpuTemp, maxDiskTemp, config)

	// Assert
	assert.Equal(t, "hdd_temp", reason)
}

// TestCheckEmergencyConditions_BothNormal tests no emergency condition
func TestCheckEmergencyConditions_BothNormal(t *testing.T) {
	// Arrange
	config := &Config{
		Temperature: TemperatureConfig{
			MaxCPU: 75.0,
			MaxHDD: 45.0,
		},
	}
	cpuTemp := 60.0   // Normal
	maxDiskTemp := 40 // Normal

	// Act
	reason := checkEmergencyConditions(cpuTemp, maxDiskTemp, config)

	// Assert
	assert.Equal(t, "", reason)
}

// TestCheckEmergencyConditions_BothOverTemp tests both over temp
func TestCheckEmergencyConditions_BothOverTemp(t *testing.T) {
	// Arrange
	config := &Config{
		Temperature: TemperatureConfig{
			MaxCPU: 75.0,
			MaxHDD: 45.0,
		},
	}
	cpuTemp := 80.0   // Above max
	maxDiskTemp := 50 // Above max

	// Act
	reason := checkEmergencyConditions(cpuTemp, maxDiskTemp, config)

	// Assert - CPU check comes first, so should return cpu_temp
	assert.Equal(t, "cpu_temp", reason)
}
