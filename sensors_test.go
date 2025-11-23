package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetAverageOfWarmest_NormalCase tests normal averaging behavior
func TestGetAverageOfWarmest_NormalCase(t *testing.T) {
	// Arrange
	temps := map[string]int{
		"sda": 35,
		"sdb": 42,
		"sdc": 38,
		"sdd": 45,
		"sde": 40,
		"sdf": 37,
	}

	// Act
	avg := GetAverageOfWarmest(temps, 3)

	// Assert - average of 3 warmest: (45 + 42 + 40) / 3 = 42.33
	assert.InDelta(t, 42.33, avg, 0.01)
}

// TestGetAverageOfWarmest_EmptyMap tests empty temperature map
func TestGetAverageOfWarmest_EmptyMap(t *testing.T) {
	// Arrange
	temps := map[string]int{}

	// Act
	avg := GetAverageOfWarmest(temps, 3)

	// Assert
	assert.Equal(t, 0.0, avg)
}

// TestGetAverageOfWarmest_FewerDisksThanN tests when fewer disks than N
func TestGetAverageOfWarmest_FewerDisksThanN(t *testing.T) {
	// Arrange
	temps := map[string]int{
		"sda": 35,
		"sdb": 40,
	}

	// Act - request average of 5 disks but only have 2
	avg := GetAverageOfWarmest(temps, 5)

	// Assert - should average all available disks: (35 + 40) / 2 = 37.5
	assert.Equal(t, 37.5, avg)
}

// TestGetAverageOfWarmest_SingleDisk tests with a single disk
func TestGetAverageOfWarmest_SingleDisk(t *testing.T) {
	// Arrange
	temps := map[string]int{
		"sda": 42,
	}

	// Act
	avg := GetAverageOfWarmest(temps, 3)

	// Assert
	assert.Equal(t, 42.0, avg)
}

// TestGetAverageOfWarmest_Sorting tests that warmest disks are selected
func TestGetAverageOfWarmest_Sorting(t *testing.T) {
	// Arrange
	temps := map[string]int{
		"sda": 30, // coldest
		"sdb": 50, // warmest
		"sdc": 35,
		"sdd": 48, // 2nd warmest
		"sde": 32,
		"sdf": 45, // 3rd warmest
	}

	// Act - get average of 3 warmest
	avg := GetAverageOfWarmest(temps, 3)

	// Assert - should average 50, 48, 45 = 47.67
	assert.InDelta(t, 47.67, avg, 0.01)
}

// TestGetMaxTemperature_NormalCase tests finding maximum temperature
func TestGetMaxTemperature_NormalCase(t *testing.T) {
	// Arrange
	temps := map[string]int{
		"sda": 35,
		"sdb": 42,
		"sdc": 38,
		"sdd": 45,
		"sde": 40,
	}

	// Act
	max := GetMaxTemperature(temps)

	// Assert
	assert.Equal(t, 45, max)
}

// TestGetMaxTemperature_EmptyMap tests empty map
func TestGetMaxTemperature_EmptyMap(t *testing.T) {
	// Arrange
	temps := map[string]int{}

	// Act
	max := GetMaxTemperature(temps)

	// Assert
	assert.Equal(t, 0, max)
}

// TestGetMaxTemperature_SingleDisk tests with single disk
func TestGetMaxTemperature_SingleDisk(t *testing.T) {
	// Arrange
	temps := map[string]int{
		"sda": 42,
	}

	// Act
	max := GetMaxTemperature(temps)

	// Assert
	assert.Equal(t, 42, max)
}

// TestGetMinTemperature_NormalCase tests finding minimum temperature
func TestGetMinTemperature_NormalCase(t *testing.T) {
	// Arrange
	temps := map[string]int{
		"sda": 35,
		"sdb": 42,
		"sdc": 38,
		"sdd": 45,
		"sde": 40,
	}

	// Act
	min := GetMinTemperature(temps)

	// Assert
	assert.Equal(t, 35, min)
}

// TestGetMinTemperature_EmptyMap tests empty map
func TestGetMinTemperature_EmptyMap(t *testing.T) {
	// Arrange
	temps := map[string]int{}

	// Act
	min := GetMinTemperature(temps)

	// Assert
	assert.Equal(t, 0, min)
}

// TestGetMinTemperature_SingleDisk tests with single disk
func TestGetMinTemperature_SingleDisk(t *testing.T) {
	// Arrange
	temps := map[string]int{
		"sda": 42,
	}

	// Act
	min := GetMinTemperature(temps)

	// Assert
	assert.Equal(t, 42, min)
}

// TestMatchesExcludePattern_ValidPatterns tests pattern matching
func TestMatchesExcludePattern_ValidPatterns(t *testing.T) {
	tests := []struct {
		name     string
		device   string
		patterns []string
		expected bool
	}{
		{
			name:     "loop device matches",
			device:   "loop0",
			patterns: []string{"^loop", "^sr"},
			expected: true,
		},
		{
			name:     "sr device matches",
			device:   "sr0",
			patterns: []string{"^loop", "^sr"},
			expected: true,
		},
		{
			name:     "zram device matches",
			device:   "zram0",
			patterns: []string{"^loop", "^zram"},
			expected: true,
		},
		{
			name:     "dm device matches",
			device:   "dm-0",
			patterns: []string{"^dm-"},
			expected: true,
		},
		{
			name:     "zd device matches",
			device:   "zd0",
			patterns: []string{"^zd"},
			expected: true,
		},
		{
			name:     "normal disk doesn't match",
			device:   "sda",
			patterns: []string{"^loop", "^sr", "^zram"},
			expected: false,
		},
		{
			name:     "nvme disk doesn't match",
			device:   "nvme0n1",
			patterns: []string{"^loop", "^sr"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := matchesExcludePattern(tt.device, tt.patterns)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMatchesExcludePattern_InvalidRegex tests handling of invalid regex
func TestMatchesExcludePattern_InvalidRegex(t *testing.T) {
	// Arrange
	device := "sda"
	patterns := []string{"[invalid", "^sr"} // Invalid regex followed by valid

	// Act - should not panic, should skip invalid pattern
	result := matchesExcludePattern(device, patterns)

	// Assert - should return false (sda doesn't match ^sr)
	assert.False(t, result)
}

// TestMatchesExcludePattern_NoMatch tests when no patterns match
func TestMatchesExcludePattern_NoMatch(t *testing.T) {
	// Arrange
	device := "sda"
	patterns := []string{"^loop", "^sr", "^zram"}

	// Act
	result := matchesExcludePattern(device, patterns)

	// Assert
	assert.False(t, result)
}

// TestMatchesExcludePattern_MultiplePatterns tests multiple pattern matching
func TestMatchesExcludePattern_MultiplePatterns(t *testing.T) {
	// Arrange
	patterns := []string{"^loop", "^sr", "^zram", "^zd", "^dm-"}

	tests := []struct {
		device   string
		expected bool
	}{
		{"loop0", true},
		{"loop15", true},
		{"sr0", true},
		{"zram0", true},
		{"zd0", true},
		{"zd128", true},
		{"dm-0", true},
		{"dm-15", true},
		{"sda", false},
		{"sdb", false},
		{"nvme0n1", false},
		{"nvme1n1", false},
	}

	for _, tt := range tests {
		t.Run(tt.device, func(t *testing.T) {
			// Act
			result := matchesExcludePattern(tt.device, patterns)

			// Assert
			assert.Equal(t, tt.expected, result, "Device: %s", tt.device)
		})
	}
}
