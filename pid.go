package main

import (
	"time"
)

// PIDController implements a PID controller with anti-windup protection
type PIDController struct {
	// PID gains
	Kp float64 // Proportional gain
	Ki float64 // Integral gain  
	Kd float64 // Derivative gain
	
	// Target setpoint
	Target float64
	
	// Internal state
	Integral    float64   // Accumulated integral term
	PrevError   float64  // Previous error for derivative calculation
	PrevTime    time.Time // Previous calculation time
	FirstRun    bool     // True on first run (skip derivative)
	
	// Output limits
	MinOutput   float64 // Minimum output value
	MaxOutput   float64 // Maximum output value
	
	// Anti-windup protection
	IntegralMax float64 // Maximum allowed integral term
}

// PIDTerms contains the individual PID components for monitoring
type PIDTerms struct {
	P     float64 // Proportional term
	I     float64 // Integral term  
	D     float64 // Derivative term
	Error float64 // Current error
}

// NewPIDController creates a new PID controller with the specified parameters
func NewPIDController(kp, ki, kd, target, minOutput, maxOutput, integralMax float64) *PIDController {
	return &PIDController{
		Kp:          kp,
		Ki:          ki,
		Kd:          kd,
		Target:      target,
		MinOutput:   minOutput,
		MaxOutput:   maxOutput,
		IntegralMax: integralMax,
		FirstRun:    true,
	}
}

// Calculate computes the PID output for the given current value
// Returns the output value and individual PID terms for monitoring
func (p *PIDController) Calculate(current float64) (float64, PIDTerms) {
	now := time.Now()
	
	// Calculate error
	error := current - p.Target
	
	// Calculate time delta (in seconds)
	var dt float64
	if !p.FirstRun {
		dt = now.Sub(p.PrevTime).Seconds()
	} else {
		dt = 1.0 // Default to 1 second on first run
	}
	
	// Proportional term
	proportional := p.Kp * error
	
	// Integral term with anti-windup
	integral := p.Integral + error*dt
	integralClamped := clamp(integral, -p.IntegralMax, p.IntegralMax)
	
	// Derivative term (skip on first run)
	var derivative float64
	if !p.FirstRun && dt > 0 {
		derivative = p.Kd * (error - p.PrevError) / dt
	}
	
	// Calculate output
	output := proportional + integralClamped + derivative
	
	// Clamp output to limits
	output = clamp(output, p.MinOutput, p.MaxOutput)
	
	// Update internal state
	p.Integral = integralClamped
	p.PrevError = error
	p.PrevTime = now
	p.FirstRun = false
	
	// Return output and terms for monitoring
	terms := PIDTerms{
		P:     proportional,
		I:     integralClamped,
		D:     derivative,
		Error: error,
	}
	
	return output, terms
}

// Reset clears the PID controller state (useful for testing or restarting)
func (p *PIDController) Reset() {
	p.Integral = 0
	p.PrevError = 0
	p.PrevTime = time.Time{}
	p.FirstRun = true
}

// SetTarget updates the target setpoint
func (p *PIDController) SetTarget(target float64) {
	p.Target = target
}

// SetGains updates the PID gains
func (p *PIDController) SetGains(kp, ki, kd float64) {
	p.Kp = kp
	p.Ki = ki
	p.Kd = kd
}

// SetLimits updates the output limits
func (p *PIDController) SetLimits(minOutput, maxOutput float64) {
	p.MinOutput = minOutput
	p.MaxOutput = maxOutput
}

// SetIntegralMax updates the integral anti-windup limit
func (p *PIDController) SetIntegralMax(integralMax float64) {
	p.IntegralMax = integralMax
}

// GetState returns the current PID controller state for debugging
func (p *PIDController) GetState() map[string]float64 {
	return map[string]float64{
		"kp":          p.Kp,
		"ki":          p.Ki,
		"kd":          p.Kd,
		"target":      p.Target,
		"integral":    p.Integral,
		"prev_error":  p.PrevError,
		"min_output":  p.MinOutput,
		"max_output":  p.MaxOutput,
		"integral_max": p.IntegralMax,
	}
}

// clamp limits a value between min and max
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// PIDTuning provides helper functions for PID tuning
type PIDTuning struct {
	controller *PIDController
}

// NewPIDTuning creates a tuning helper for the given controller
func NewPIDTuning(controller *PIDController) *PIDTuning {
	return &PIDTuning{controller: controller}
}

// TuneForTemperatureControl provides recommended PID gains for temperature control
// These are starting values that may need adjustment based on system behavior
func (t *PIDTuning) TuneForTemperatureControl() {
	// Conservative starting values for temperature control
	// These are based on the legacy script values but may need tuning
	t.controller.SetGains(5.0, 0.1, 20.0)
	t.controller.SetIntegralMax(50.0)
}

// TuneForResponsiveControl provides more aggressive gains for faster response
// Use with caution - may cause oscillation
func (t *PIDTuning) TuneForResponsiveControl() {
	// More aggressive values for faster response
	t.controller.SetGains(8.0, 0.2, 30.0)
	t.controller.SetIntegralMax(75.0)
}

// TuneForStableControl provides conservative gains for stable operation
// Slower response but less likely to oscillate
func (t *PIDTuning) TuneForStableControl() {
	// Conservative values for stable operation
	t.controller.SetGains(3.0, 0.05, 15.0)
	t.controller.SetIntegralMax(25.0)
}

// ValidateGains checks if the current gains are reasonable for temperature control
func (t *PIDTuning) ValidateGains() []string {
	var warnings []string
	
	// Check for reasonable ranges
	if t.controller.Kp < 0 || t.controller.Kp > 20 {
		warnings = append(warnings, "Kp should typically be between 0-20")
	}
	
	if t.controller.Ki < 0 || t.controller.Ki > 2 {
		warnings = append(warnings, "Ki should typically be between 0-2")
	}
	
	if t.controller.Kd < 0 || t.controller.Kd > 100 {
		warnings = append(warnings, "Kd should typically be between 0-100")
	}
	
	// Check for potential oscillation
	if t.controller.Kp > 10 && t.controller.Ki > 0.5 {
		warnings = append(warnings, "High Kp with high Ki may cause oscillation")
	}
	
	return warnings
}
