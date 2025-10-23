package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the fan controller
type Metrics struct {
	// Temperature metrics
	HDDTemperature     *prometheus.GaugeVec   // Individual disk temperatures
	HDDTemperatureMax  prometheus.Gauge      // Maximum disk temperature
	HDDTemperatureAvg  prometheus.Gauge      // Average of warmest disks
	CPUTemperature     prometheus.Gauge      // CPU temperature
	
	// Fan metrics
	FanDutyPercent     prometheus.Gauge      // Current fan duty cycle
	FanSpeedRPM        *prometheus.GaugeVec // Individual fan speeds
	
	// PID metrics
	PIDProportional    prometheus.Gauge      // P term
	PIDIntegral        prometheus.Gauge      // I term
	PIDDerivative      prometheus.Gauge      // D term
	PIDError           prometheus.Gauge      // Current error
	
	// System metrics
	EmergencyMode      *prometheus.GaugeVec // Emergency mode status
	ErrorsTotal        *prometheus.CounterVec // Error counters
	LoopDuration       prometheus.Histogram // Control loop timing
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
}

var (
	// Global metrics instance
	metrics *Metrics
	
	// Start time for uptime calculation
	startTime time.Time
)

// InitMetrics initializes all Prometheus metrics
func InitMetrics() *Metrics {
	startTime = time.Now()
	
	metrics = &Metrics{
		// Temperature metrics
		HDDTemperature: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "fan_controller_hdd_temperature_celsius",
				Help: "HDD temperature in Celsius",
			},
			[]string{"disk"},
		),
		HDDTemperatureMax: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "fan_controller_hdd_temperature_max_celsius",
				Help: "Maximum HDD temperature in Celsius",
			},
		),
		HDDTemperatureAvg: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "fan_controller_hdd_temperature_avg_celsius",
				Help: "Average temperature of warmest disks in Celsius",
			},
		),
		CPUTemperature: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "fan_controller_cpu_temperature_celsius",
				Help: "CPU temperature in Celsius",
			},
		),
		
		// Fan metrics
		FanDutyPercent: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "fan_controller_fan_duty_percent",
				Help: "Current fan duty cycle percentage",
			},
		),
		FanSpeedRPM: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "fan_controller_fan_speed_rpm",
				Help: "Fan speed in RPM",
			},
			[]string{"fan"},
		),
		
		// PID metrics
		PIDProportional: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "fan_controller_pid_proportional",
				Help: "PID proportional term",
			},
		),
		PIDIntegral: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "fan_controller_pid_integral",
				Help: "PID integral term",
			},
		),
		PIDDerivative: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "fan_controller_pid_derivative",
				Help: "PID derivative term",
			},
		),
		PIDError: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "fan_controller_pid_error_celsius",
				Help: "PID error in Celsius",
			},
		),
		
		// System metrics
		EmergencyMode: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "fan_controller_emergency_mode",
				Help: "Emergency mode status (1=active, 0=normal)",
			},
			[]string{"reason"},
		),
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fan_controller_errors_total",
				Help: "Total number of errors by type",
			},
			[]string{"type"},
		),
		LoopDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name: "fan_controller_loop_duration_seconds",
				Help: "Control loop execution time in seconds",
				Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
			},
		),
	}
	
	// Register all metrics
	prometheus.MustRegister(
		metrics.HDDTemperature,
		metrics.HDDTemperatureMax,
		metrics.HDDTemperatureAvg,
		metrics.CPUTemperature,
		metrics.FanDutyPercent,
		metrics.FanSpeedRPM,
		metrics.PIDProportional,
		metrics.PIDIntegral,
		metrics.PIDDerivative,
		metrics.PIDError,
		metrics.EmergencyMode,
		metrics.ErrorsTotal,
		metrics.LoopDuration,
	)
	
	return metrics
}

// StartMetricsServer starts the HTTP server for Prometheus metrics
func StartMetricsServer(port int) error {
	// Health check endpoint
	http.HandleFunc("/health", healthHandler)
	
	// Prometheus metrics endpoint
	http.Handle("/metrics", promhttp.Handler())
	
	// Start server in goroutine
	go func() {
		addr := fmt.Sprintf(":%d", port)
		log.Printf("Starting metrics server on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Metrics server error: %v", err)
		}
	}()
	
	return nil
}

// healthHandler provides a health check endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(startTime)
	
	response := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Uptime:    uptime.String(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode health response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// UpdateAllMetrics updates all metrics with current values
func UpdateAllMetrics(
	diskTemps map[string]int,
	cpuTemp float64,
	fanSpeeds map[string]int,
	fanDuty int,
	pidTerms PIDTerms,
	avgTemp float64,
	maxTemp int,
	emergencyReason string,
	loopDuration time.Duration,
) {
	// Update disk temperatures
	for disk, temp := range diskTemps {
		metrics.HDDTemperature.WithLabelValues(disk).Set(float64(temp))
	}
	
	// Update temperature summaries
	metrics.HDDTemperatureMax.Set(float64(maxTemp))
	metrics.HDDTemperatureAvg.Set(avgTemp)
	metrics.CPUTemperature.Set(cpuTemp)
	
	// Update fan metrics
	metrics.FanDutyPercent.Set(float64(fanDuty))
	for fan, speed := range fanSpeeds {
		metrics.FanSpeedRPM.WithLabelValues(fan).Set(float64(speed))
	}
	
	// Update PID metrics
	metrics.PIDProportional.Set(pidTerms.P)
	metrics.PIDIntegral.Set(pidTerms.I)
	metrics.PIDDerivative.Set(pidTerms.D)
	metrics.PIDError.Set(pidTerms.Error)
	
	// Update emergency mode
	if emergencyReason != "" {
		metrics.EmergencyMode.WithLabelValues(emergencyReason).Set(1)
		// Reset other emergency reasons to 0
		metrics.EmergencyMode.WithLabelValues("hdd_temp").Set(0)
		metrics.EmergencyMode.WithLabelValues("cpu_temp").Set(0)
	} else {
		// No emergency - reset all
		metrics.EmergencyMode.WithLabelValues("hdd_temp").Set(0)
		metrics.EmergencyMode.WithLabelValues("cpu_temp").Set(0)
	}
	
	// Update loop duration
	metrics.LoopDuration.Observe(loopDuration.Seconds())
}

// RecordError increments the error counter for the specified type
func RecordError(errorType string) {
	metrics.ErrorsTotal.WithLabelValues(errorType).Inc()
}

// GetMetrics returns the global metrics instance
func GetMetrics() *Metrics {
	return metrics
}

// ResetMetrics resets all metrics to zero (useful for testing)
func ResetMetrics() {
	// Reset gauges
	metrics.HDDTemperature.Reset()
	metrics.HDDTemperatureMax.Set(0)
	metrics.HDDTemperatureAvg.Set(0)
	metrics.CPUTemperature.Set(0)
	metrics.FanDutyPercent.Set(0)
	metrics.FanSpeedRPM.Reset()
	metrics.PIDProportional.Set(0)
	metrics.PIDIntegral.Set(0)
	metrics.PIDDerivative.Set(0)
	metrics.PIDError.Set(0)
	metrics.EmergencyMode.Reset()
	
	// Note: Counters and histograms are not reset as they are cumulative
}

// MetricsSummary provides a summary of current metric values for logging
type MetricsSummary struct {
	CPUTemp      float64
	MaxDiskTemp  int
	AvgDiskTemp  float64
	FanDuty      int
	PIDError     float64
	Emergency    string
	LoopTime     time.Duration
}

// GetMetricsSummary returns a summary of current metrics for logging
func GetMetricsSummary(
	diskTemps map[string]int,
	cpuTemp float64,
	fanDuty int,
	pidTerms PIDTerms,
	avgTemp float64,
	maxTemp int,
	emergencyReason string,
	loopDuration time.Duration,
) MetricsSummary {
	return MetricsSummary{
		CPUTemp:     cpuTemp,
		MaxDiskTemp: maxTemp,
		AvgDiskTemp: avgTemp,
		FanDuty:     fanDuty,
		PIDError:    pidTerms.Error,
		Emergency:   emergencyReason,
		LoopTime:    loopDuration,
	}
}

// LogMetricsSummary logs a formatted summary of current metrics
func LogMetricsSummary(summary MetricsSummary) {
	if summary.Emergency != "" {
		log.Printf("EMERGENCY: %s | CPU: %.1f°C | Max: %d°C | Avg: %.1f°C | Duty: %d%% | Error: %.1f°C | Time: %v",
			summary.Emergency, summary.CPUTemp, summary.MaxDiskTemp, 
			summary.AvgDiskTemp, summary.FanDuty, summary.PIDError, summary.LoopTime)
	} else {
		log.Printf("Status: CPU: %.1f°C | Max: %d°C | Avg: %.1f°C | Duty: %d%% | Error: %.1f°C | Time: %v",
			summary.CPUTemp, summary.MaxDiskTemp, summary.AvgDiskTemp, 
			summary.FanDuty, summary.PIDError, summary.LoopTime)
	}
}
