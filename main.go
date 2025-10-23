package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	// CLI flags
	configPath = flag.String("config", "/config/config.yaml", "Path to configuration file")
	dryRun     = flag.Bool("dry-run", false, "Run in dry-run mode (no IPMI commands)")
	testIPMI   = flag.Bool("test-ipmi", false, "Test IPMI functionality and exit")
	logLevel   = flag.String("log-level", "", "Override log level (debug, info, warn, error)")
)

func main() {
	flag.Parse()
	
	// Load configuration
	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	// Override log level if specified
	if *logLevel != "" {
		config.Server.LogLevel = *logLevel
	}
	
	log.Printf("Starting fan controller (config: %s)", *configPath)
	
	// Handle test-ipmi flag
	if *testIPMI {
		if err := TestIPMICommand(); err != nil {
			log.Fatalf("IPMI test failed: %v", err)
		}
		log.Println("IPMI test completed successfully")
		return
	}
	
	// Initialize metrics
	metrics := InitMetrics()
	
	// Start metrics server
	if err := StartMetricsServer(config.Server.MetricsPort); err != nil {
		log.Fatalf("Failed to start metrics server: %v", err)
	}
	
	// Initialize PID controller
	pid := NewPIDController(
		config.PID.Kp,
		config.PID.Ki,
		config.PID.Kd,
		config.Temperature.TargetHDD,
		float64(config.Fans.MinDuty),
		float64(config.Fans.MaxDuty),
		config.PID.IntegralMax,
	)
	
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Start control loop in goroutine
	controlLoopDone := make(chan bool)
	go func() {
		runControlLoop(config, pid, metrics)
		controlLoopDone <- true
	}()
	
	// Wait for shutdown signal
	<-sigChan
	log.Println("Received shutdown signal, setting fans to 100%...")
	
	// Emergency shutdown: set fans to 100%
	if !*dryRun {
		if err := SetAllFans(100); err != nil {
			log.Printf("Warning: failed to set fans to 100%% during shutdown: %v", err)
		} else {
			log.Println("Fans set to 100% for safety")
		}
	}
	
	// Wait for control loop to finish
	<-controlLoopDone
	log.Println("Fan controller stopped")
}

// runControlLoop executes the main control loop
func runControlLoop(config *Config, pid *PIDController, metrics *Metrics) {
	log.Printf("Starting control loop (target: %.1f°C, interval: %v)", 
		config.Temperature.TargetHDD, config.Temperature.PollInterval)
	
	// Set initial fan speed
	if !*dryRun {
		if err := SetAllFans(config.Fans.StartupDuty); err != nil {
			log.Printf("Warning: failed to set initial fan speed: %v", err)
		} else {
			log.Printf("Set initial fan speed to %d%%", config.Fans.StartupDuty)
		}
	}
	
	// Control loop state
	var consecutiveIPMIFailures int
	const maxIPMIFailures = 5
	
	// Main control loop
	for {
		loopStart := time.Now()
		
		// Read temperatures
		diskTemps, cpuTemp, err := readAllTemperatures(config)
		if err != nil {
			log.Printf("Error reading temperatures: %v", err)
			RecordError("temperature")
			time.Sleep(config.Temperature.PollInterval)
			continue
		}
		
		// Calculate temperature metrics
		avgTemp := GetAverageOfWarmest(diskTemps, config.Temperature.WarmestDisks)
		maxTemp := GetMaxTemperature(diskTemps)
		
		// Check for emergency conditions
		emergencyReason := checkEmergencyConditions(cpuTemp, maxTemp, config)
		
		var fanDuty int
		var pidTerms PIDTerms
		
		if emergencyReason != "" {
			// Emergency mode: set fans to 100%
			fanDuty = 100
			pidTerms = PIDTerms{} // Zero terms in emergency
			log.Printf("EMERGENCY: %s - setting fans to 100%%", emergencyReason)
		} else {
			// Normal PID control
			output, terms := pid.Calculate(avgTemp)
			pidTerms = terms
			fanDuty = int(output)
			
			// Clamp to fan limits
			if fanDuty < config.Fans.MinDuty {
				fanDuty = config.Fans.MinDuty
			}
			if fanDuty > config.Fans.MaxDuty {
				fanDuty = config.Fans.MaxDuty
			}
		}
		
		// Set fan speed (unless in dry-run mode)
		if !*dryRun {
			if err := SetAllFans(fanDuty); err != nil {
				consecutiveIPMIFailures++
				RecordError("ipmi")
				log.Printf("IPMI command failed (attempt %d/%d): %v", 
					consecutiveIPMIFailures, maxIPMIFailures, err)
				
				// If too many consecutive failures, force emergency mode
				if consecutiveIPMIFailures >= maxIPMIFailures {
					log.Printf("Too many IPMI failures (%d), forcing emergency mode", consecutiveIPMIFailures)
					emergencyReason = "ipmi_failure"
					fanDuty = 100
					// Try one more time to set 100%
					if err := SetAllFans(100); err != nil {
						log.Printf("Critical: failed to set emergency fan speed: %v", err)
					}
				}
			} else {
				consecutiveIPMIFailures = 0 // Reset failure counter on success
			}
		}
		
		// Read current fan speeds for metrics
		fanSpeeds, err := GetFanSpeeds()
		if err != nil {
			log.Printf("Warning: failed to read fan speeds: %v", err)
			fanSpeeds = make(map[string]int) // Empty map for metrics
		}
		
		// Update metrics
		UpdateAllMetrics(
			diskTemps, cpuTemp, fanSpeeds, fanDuty,
			pidTerms, avgTemp, maxTemp, emergencyReason,
			time.Since(loopStart),
		)
		
		// Log status
		summary := GetMetricsSummary(
			diskTemps, cpuTemp, fanDuty, pidTerms,
			avgTemp, maxTemp, emergencyReason, time.Since(loopStart),
		)
		LogMetricsSummary(summary)
		
		// Sleep until next iteration
		time.Sleep(config.Temperature.PollInterval)
	}
}

// readAllTemperatures reads all temperature sensors
func readAllTemperatures(config *Config) (map[string]int, float64, error) {
	// Read disk temperatures
	diskTemps, err := GetAllDiskTemperatures(config.Disks.ExcludePatterns)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read disk temperatures: %w", err)
	}
	
	// Read CPU temperature
	cpuTemp, err := GetCPUTemperature()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read CPU temperature: %w", err)
	}
	
	return diskTemps, cpuTemp, nil
}

// checkEmergencyConditions checks for emergency temperature conditions
func checkEmergencyConditions(cpuTemp float64, maxDiskTemp int, config *Config) string {
	// Check CPU emergency temperature
	if cpuTemp > config.Temperature.MaxCPU {
		return "cpu_temp"
	}
	
	// Check disk emergency temperature
	if maxDiskTemp > config.Temperature.MaxHDD {
		return "hdd_temp"
	}
	
	return "" // No emergency
}

// validateEnvironment checks if the environment is suitable for operation
func validateEnvironment(config *Config) error {
	// Check if we can read CPU temperature
	if _, err := GetCPUTemperature(); err != nil {
		return fmt.Errorf("CPU temperature sensor not accessible: %w", err)
	}
	
	// Check if we can read disk temperatures
	diskTemps, err := GetAllDiskTemperatures(config.Disks.ExcludePatterns)
	if err != nil {
		return fmt.Errorf("disk temperature sensors not accessible: %w", err)
	}
	
	if len(diskTemps) == 0 {
		return fmt.Errorf("no spinning disks found for temperature monitoring")
	}
	
	// Check IPMI accessibility (unless in dry-run mode)
	if !*dryRun {
		if _, err := GetFanSpeeds(); err != nil {
			return fmt.Errorf("IPMI not accessible: %w", err)
		}
	}
	
	log.Printf("Environment validation passed: %d disks, CPU sensor OK", len(diskTemps))
	return nil
}
