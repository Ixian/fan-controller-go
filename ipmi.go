package main

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// IPMI command format for ASRock X570D4U-2L2T (confirmed via testing)
	ipmiFormat = "0xd6"
	numFans    = 6
	numPadding = 10
)

// SetAllFans sets the duty cycle for all fans using the confirmed 0xd6 format
// ASRock X570D4U-2L2T uses format: ipmitool raw 0x3a 0xd6 [6 fan values] [10 padding bytes]
func SetAllFans(dutyPercent int) error {
	if dutyPercent < 0 || dutyPercent > 100 {
		return fmt.Errorf("duty cycle must be between 0-100, got %d", dutyPercent)
	}

	// Convert duty percentage to hex (0-100 -> 0x00-0x64)
	dutyHex := fmt.Sprintf("0x%02x", dutyPercent)
	
	// Build command: 0x3a 0xd6 [6 fan values] [10 padding bytes at 0x64]
	args := []string{"raw", "0x3a", "0xd6"}
	
	// Add 6 fan duty values (all set to same duty cycle)
	for i := 0; i < numFans; i++ {
		args = append(args, dutyHex)
	}
	
	// Add 10 padding bytes (always 0x64 = 100 decimal)
	for i := 0; i < numPadding; i++ {
		args = append(args, "0x64")
	}

	// Execute with retries (3 attempts, 2 second delays)
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		cmd := exec.Command("ipmitool", args...)
		output, err := cmd.CombinedOutput()
		
		if err == nil {
			// Success - no need to retry
			return nil
		}
		
		lastErr = fmt.Errorf("attempt %d failed: %v, output: %s", attempt, err, string(output))
		
		if attempt < 3 {
			log.Printf("IPMI command failed, retrying in 2s: %v", lastErr)
			time.Sleep(2 * time.Second)
		}
	}
	
	return fmt.Errorf("IPMI command failed after 3 attempts: %w", lastErr)
}

// GetFanSpeeds reads current fan speeds from IPMI sensors
// Returns a map of fan name -> RPM, or error if reading fails
func GetFanSpeeds() (map[string]int, error) {
	cmd := exec.Command("ipmitool", "sensor")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to read IPMI sensors: %w", err)
	}

	fanSpeeds := make(map[string]int)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	
	// Regex to match fan sensor lines: "FAN1 | 1600.000 | RPM | ok | ..."
	fanRegex := regexp.MustCompile(`^(FAN\w+)\s*\|\s*([0-9.]+|na)\s*\|\s*RPM`)
	
	for scanner.Scan() {
		line := scanner.Text()
		matches := fanRegex.FindStringSubmatch(line)
		
		if len(matches) == 3 {
			fanName := matches[1]
			rpmStr := matches[2]
			
			// Skip fans with "na" reading (no sensor connected)
			if rpmStr == "na" {
				continue
			}
			
			// Parse RPM value
			rpm, err := strconv.ParseFloat(rpmStr, 64)
			if err != nil {
				log.Printf("Warning: failed to parse RPM for %s: %v", fanName, err)
				continue
			}
			
			fanSpeeds[fanName] = int(rpm)
		}
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse IPMI sensor output: %w", err)
	}
	
	if len(fanSpeeds) == 0 {
		return nil, fmt.Errorf("no fan sensors found in IPMI output")
	}
	
	return fanSpeeds, nil
}

// GetFanSpeedsForLogging returns fan speeds formatted for logging
// Returns a string like "FAN1:1600 FAN2:1700 FAN3:2900" for easy reading
func GetFanSpeedsForLogging() string {
	speeds, err := GetFanSpeeds()
	if err != nil {
		return fmt.Sprintf("Error reading fan speeds: %v", err)
	}
	
	var parts []string
	for fan, rpm := range speeds {
		parts = append(parts, fmt.Sprintf("%s:%d", fan, rpm))
	}
	
	return strings.Join(parts, " ")
}

// TestIPMICommand tests the IPMI command format and returns results
// This is used by the --test-ipmi CLI flag to verify IPMI functionality
func TestIPMICommand() error {
	log.Println("Testing IPMI command format 0xd6...")
	
	// Get baseline fan speeds
	log.Println("Getting baseline fan speeds...")
	baseline, err := GetFanSpeeds()
	if err != nil {
		return fmt.Errorf("failed to get baseline fan speeds: %w", err)
	}
	log.Printf("Baseline speeds: %s", GetFanSpeedsForLogging())
	
	// Test setting to 50% duty cycle
	log.Println("Setting fans to 50% duty cycle...")
	if err := SetAllFans(50); err != nil {
		return fmt.Errorf("failed to set fans to 50%%: %w", err)
	}
	
	// Wait for fans to adjust
	log.Println("Waiting 10 seconds for fans to adjust...")
	time.Sleep(10 * time.Second)
	
	// Check fan speeds after adjustment
	log.Println("Checking fan speeds after adjustment...")
	adjusted, err := GetFanSpeeds()
	if err != nil {
		return fmt.Errorf("failed to get adjusted fan speeds: %w", err)
	}
	log.Printf("Adjusted speeds: %s", GetFanSpeedsForLogging())
	
	// Verify speeds changed (should be roughly 50% of baseline)
	changesDetected := 0
	for fan, newRPM := range adjusted {
		if baselineRPM, exists := baseline[fan]; exists {
			// Allow for some variation (40-60% of baseline)
			expectedMin := int(float64(baselineRPM) * 0.4)
			expectedMax := int(float64(baselineRPM) * 0.6)
			
			if newRPM >= expectedMin && newRPM <= expectedMax {
				changesDetected++
				log.Printf("✓ %s: %d -> %d RPM (%.1f%% change)", 
					fan, baselineRPM, newRPM, 
					float64(newRPM)/float64(baselineRPM)*100)
			} else {
				log.Printf("⚠ %s: %d -> %d RPM (unexpected change)", 
					fan, baselineRPM, newRPM)
			}
		}
	}
	
	if changesDetected == 0 {
		return fmt.Errorf("no fan speed changes detected - IPMI command may not be working")
	}
	
	log.Printf("✓ IPMI test successful: %d fans responded to command", changesDetected)
	
	// Reset to 100% for safety
	log.Println("Resetting fans to 100% duty cycle...")
	if err := SetAllFans(100); err != nil {
		return fmt.Errorf("failed to reset fans to 100%%: %w", err)
	}
	
	// Wait and verify reset
	log.Println("Waiting 10 seconds for fans to return to 100%...")
	time.Sleep(10 * time.Second)
	
	_, err := GetFanSpeeds()
	if err != nil {
		return fmt.Errorf("failed to get final fan speeds: %w", err)
	}
	log.Printf("Final speeds: %s", GetFanSpeedsForLogging())
	
	log.Println("✓ IPMI test completed successfully")
	return nil
}
