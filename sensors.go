package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Cache the k10temp hwmon path after first successful detection
	cachedK10TempPath string
)

// GetCPUTemperature reads CPU temperature from k10temp sensor
// Auto-detects the hwmon path and caches it for subsequent calls
func GetCPUTemperature() (float64, error) {
	// Use cached path if available
	if cachedK10TempPath != "" {
		return readCPUTempFromPath(cachedK10TempPath)
	}
	
	// Auto-detect k10temp hwmon path
	hwmonPath, err := findK10TempPath()
	if err != nil {
		return 0, fmt.Errorf("failed to find k10temp sensor: %w", err)
	}
	
	// Cache the path for future calls
	cachedK10TempPath = hwmonPath
	
	return readCPUTempFromPath(hwmonPath)
}

// findK10TempPath searches for k10temp in /sys/class/hwmon/hwmon*/name
func findK10TempPath() (string, error) {
	matches, err := filepath.Glob("/sys/class/hwmon/hwmon*/name")
	if err != nil {
		return "", fmt.Errorf("failed to search hwmon directories: %w", err)
	}
	
	for _, namePath := range matches {
		content, err := os.ReadFile(namePath)
		if err != nil {
			continue // Skip files we can't read
		}
		
		if strings.TrimSpace(string(content)) == "k10temp" {
			// Found k10temp! Extract the hwmon directory path
			hwmonDir := filepath.Dir(namePath)
			return hwmonDir, nil
		}
	}
	
	return "", fmt.Errorf("k10temp sensor not found in /sys/class/hwmon/")
}

// readCPUTempFromPath reads temperature from the specified hwmon path
func readCPUTempFromPath(hwmonPath string) (float64, error) {
	tempPath := filepath.Join(hwmonPath, "temp1_input")
	data, err := os.ReadFile(tempPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read CPU temperature from %s: %w", tempPath, err)
	}
	
	// Parse millidegrees and convert to degrees
	millidegrees, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse CPU temperature: %w", err)
	}
	
	// Convert millidegrees to degrees Celsius
	return float64(millidegrees) / 1000.0, nil
}

// GetDiskTemperature reads temperature from a single disk using smartctl
// Handles both SATA and NVMe disks with different parsing logic
func GetDiskTemperature(device string) (int, error) {
	cmd := exec.Command("smartctl", "-A", fmt.Sprintf("/dev/%s", device))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("smartctl failed for %s: %w", device, err)
	}
	
	// Determine if this is an NVMe device
	isNVMe := strings.HasPrefix(device, "nvme")
	
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		
		if isNVMe {
			// NVMe format: "Temperature:                        33 Celsius"
			if strings.HasPrefix(line, "Temperature:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					temp, err := strconv.Atoi(fields[1])
					if err != nil {
						continue
					}
					return temp, nil
				}
			}
		} else {
			// SATA format: "194 Temperature_Celsius     0x0002   171   171   000    Old_age   Always       -       38 (Min/Max 11/51)"
			if strings.Contains(line, "Temperature_Celsius") {
				fields := strings.Fields(line)
				if len(fields) >= 10 {
					temp, err := strconv.Atoi(fields[9]) // 10th field (0-indexed = 9)
					if err != nil {
						continue
					}
					return temp, nil
				}
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("failed to parse smartctl output for %s: %w", device, err)
	}
	
	return 0, fmt.Errorf("no temperature found for device %s", device)
}

// GetAllDiskTemperatures auto-discovers spinning disks and reads their temperatures
// Uses ROTA=1 filtering and exclude patterns to identify relevant disks
func GetAllDiskTemperatures(excludePatterns []string) (map[string]int, error) {
	// Discover spinning disks
	disks, err := discoverSpinningDisks(excludePatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to discover spinning disks: %w", err)
	}
	
	if len(disks) == 0 {
		return nil, fmt.Errorf("no spinning disks found")
	}
	
	// Read temperatures for each disk
	temps := make(map[string]int)
	var errors []string
	
	for _, disk := range disks {
		temp, err := GetDiskTemperature(disk)
		if err != nil {
			log.Printf("Warning: failed to read temperature for %s: %v", disk, err)
			errors = append(errors, fmt.Sprintf("%s: %v", disk, err))
			continue
		}
		
		temps[disk] = temp
	}
	
	// If we couldn't read any temperatures, return an error
	if len(temps) == 0 {
		return nil, fmt.Errorf("failed to read temperatures from any disk: %s", strings.Join(errors, "; "))
	}
	
	// Log any partial failures
	if len(errors) > 0 {
		log.Printf("Partial disk temperature reading failures: %s", strings.Join(errors, "; "))
	}
	
	return temps, nil
}

// discoverSpinningDisks finds all spinning disks by checking /sys/block/
func discoverSpinningDisks(excludePatterns []string) ([]string, error) {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, fmt.Errorf("failed to read /sys/block: %w", err)
	}
	
	var disks []string
	
	for _, entry := range entries {
		device := entry.Name()
		
		// Skip if matches exclude patterns
		if matchesExcludePattern(device, excludePatterns) {
			continue
		}
		
		// Check if it's a spinning disk (ROTA=1) and not removable
		isSpinning, err := isSpinningDisk(device)
		if err != nil {
			log.Printf("Warning: failed to check if %s is spinning: %v", device, err)
			continue
		}
		
		if isSpinning {
			disks = append(disks, device)
		}
	}
	
	return disks, nil
}

// isSpinningDisk checks if a device is a spinning disk
func isSpinningDisk(device string) (bool, error) {
	// Check rotational flag (must be 1 for spinning disks)
	rotaPath := fmt.Sprintf("/sys/block/%s/queue/rotational", device)
	rotaData, err := os.ReadFile(rotaPath)
	if err != nil {
		return false, err
	}
	
	if strings.TrimSpace(string(rotaData)) != "1" {
		return false, nil
	}
	
	// Check removable flag (must be 0 for internal disks)
	removablePath := fmt.Sprintf("/sys/block/%s/removable", device)
	removableData, err := os.ReadFile(removablePath)
	if err != nil {
		return false, err
	}
	
	if strings.TrimSpace(string(removableData)) != "0" {
		return false, nil
	}
	
	return true, nil
}

// matchesExcludePattern checks if a device name matches any exclude pattern
func matchesExcludePattern(device string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := regexp.MatchString(pattern, device)
		if err != nil {
			log.Printf("Warning: invalid exclude pattern %s: %v", pattern, err)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// GetAverageOfWarmest calculates the average temperature of the N warmest disks
func GetAverageOfWarmest(temps map[string]int, n int) float64 {
	if len(temps) == 0 {
		return 0.0
	}
	
	// Convert to slice and sort by temperature (descending)
	var tempSlice []int
	for _, temp := range temps {
		tempSlice = append(tempSlice, temp)
	}
	
	// Simple bubble sort (descending) - fine for small number of disks
	for i := 0; i < len(tempSlice)-1; i++ {
		for j := 0; j < len(tempSlice)-i-1; j++ {
			if tempSlice[j] < tempSlice[j+1] {
				tempSlice[j], tempSlice[j+1] = tempSlice[j+1], tempSlice[j]
			}
		}
	}
	
	// Take the top N (or all if fewer than N)
	take := n
	if len(tempSlice) < take {
		take = len(tempSlice)
	}
	
	// Calculate average
	sum := 0
	for i := 0; i < take; i++ {
		sum += tempSlice[i]
	}
	
	return float64(sum) / float64(take)
}

// GetMaxTemperature returns the highest temperature from the map
func GetMaxTemperature(temps map[string]int) int {
	max := 0
	for _, temp := range temps {
		if temp > max {
			max = temp
		}
	}
	return max
}

// GetMinTemperature returns the lowest temperature from the map
func GetMinTemperature(temps map[string]int) int {
	if len(temps) == 0 {
		return 0
	}
	
	min := temps[func() string {
		for k := range temps {
			return k
		}
		return ""
	}()]
	
	for _, temp := range temps {
		if temp < min {
			min = temp
		}
	}
	return min
}
