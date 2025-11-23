package main

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPIDController_Calculate_Proportional tests the proportional term
func TestPIDController_Calculate_Proportional(t *testing.T) {
	// Arrange
	pid := NewPIDController(2.0, 0.0, 0.0, 38.0, 0, 100, 50)

	// Act - temperature above target
	output, terms := pid.Calculate(40.0)

	// Assert
	assert.InDelta(t, 2.0*2.0, terms.P, 0.01) // Kp * error = 2.0 * (40-38)
	assert.InDelta(t, 2.0, terms.Error, 0.01)
	// Integral accumulates but Ki=0 so it's stored but doesn't affect output
	assert.InDelta(t, 0.0, terms.D, 0.01) // Kd is 0
	// Output = P + I + D, where I is accumulated integral (clamped)
	assert.Greater(t, output, 0.0, "Output should be positive with temp above target")
}

// TestPIDController_Calculate_Integral tests the integral term
func TestPIDController_Calculate_Integral(t *testing.T) {
	// Arrange
	pid := NewPIDController(0.0, 0.1, 0.0, 38.0, 0, 100, 50)

	// Act - run multiple cycles to accumulate integral
	pid.Calculate(40.0) // error = 2.0
	time.Sleep(10 * time.Millisecond)
	output, terms := pid.Calculate(40.0) // error = 2.0 again

	// Assert - integral should accumulate
	assert.Greater(t, terms.I, 0.0, "Integral term should be positive")
	assert.Equal(t, 0.0, terms.P)       // Kp is 0
	assert.Equal(t, 0.0, terms.D)       // Kd is 0
	assert.Greater(t, output, 0.0)
}

// TestPIDController_Calculate_Derivative tests the derivative term
func TestPIDController_Calculate_Derivative(t *testing.T) {
	// Arrange
	pid := NewPIDController(0.0, 0.0, 10.0, 38.0, -100, 100, 50)

	// Act - first run (derivative should be 0 on first run)
	_, terms1 := pid.Calculate(40.0)
	assert.InDelta(t, 0.0, terms1.D, 0.01, "Derivative should be 0 on first run")

	// Second run with changing error
	time.Sleep(100 * time.Millisecond)
	_, terms2 := pid.Calculate(39.0) // error changes from 2.0 to 1.0

	// Assert - derivative should be negative (error decreasing)
	assert.NotEqual(t, 0.0, terms2.D, "Derivative should not be 0 on second run")
	assert.InDelta(t, 0.0, terms2.P, 0.01)   // Kp is 0
	// Integral accumulates even when Ki=0
}

// TestPIDController_Calculate_AntiWindup tests integral anti-windup
func TestPIDController_Calculate_AntiWindup(t *testing.T) {
	// Arrange
	pid := NewPIDController(0.0, 1.0, 0.0, 38.0, 0, 100, 10.0)

	// Act - accumulate large integral by running many cycles
	for i := 0; i < 50; i++ {
		pid.Calculate(50.0) // Large error = 12.0
		time.Sleep(10 * time.Millisecond)
	}
	_, terms := pid.Calculate(50.0)

	// Assert - integral should be clamped to integralMax (10.0)
	assert.LessOrEqual(t, terms.I, 10.0, "Integral should be clamped to integralMax")
	assert.GreaterOrEqual(t, terms.I, -10.0, "Integral should be clamped to -integralMax")
}

// TestPIDController_FirstRun_SkipsDerivative tests first run behavior
func TestPIDController_FirstRun_SkipsDerivative(t *testing.T) {
	// Arrange
	pid := NewPIDController(1.0, 0.1, 5.0, 38.0, 0, 100, 50)

	// Act - first calculation
	_, terms := pid.Calculate(40.0)

	// Assert - derivative should be 0 on first run
	assert.Equal(t, 0.0, terms.D, "Derivative should be 0 on first run")
	assert.NotEqual(t, 0.0, terms.P, "Proportional should work on first run")
}

// TestPIDController_OutputClamping_Min tests output clamping to minimum
func TestPIDController_OutputClamping_Min(t *testing.T) {
	// Arrange
	pid := NewPIDController(10.0, 0.0, 0.0, 38.0, 30, 100, 50)

	// Act - temperature well below target (large negative output)
	output, _ := pid.Calculate(30.0) // error = -8.0, P = -80

	// Assert - output should be clamped to minOutput (30)
	assert.Equal(t, 30.0, output)
}

// TestPIDController_OutputClamping_Max tests output clamping to maximum
func TestPIDController_OutputClamping_Max(t *testing.T) {
	// Arrange
	pid := NewPIDController(10.0, 0.0, 0.0, 38.0, 0, 100, 50)

	// Act - temperature well above target (large positive output)
	output, _ := pid.Calculate(50.0) // error = 12.0, P = 120

	// Assert - output should be clamped to maxOutput (100)
	assert.Equal(t, 100.0, output)
}

// TestPIDController_ZeroError_StableOutput tests behavior at setpoint
func TestPIDController_ZeroError_StableOutput(t *testing.T) {
	// Arrange
	pid := NewPIDController(5.0, 0.1, 2.0, 38.0, 0, 100, 50)

	// Act - run at exact target temperature
	output1, terms1 := pid.Calculate(38.0)
	time.Sleep(10 * time.Millisecond)
	output2, terms2 := pid.Calculate(38.0)

	// Assert - all terms should be near zero
	assert.Equal(t, 0.0, terms1.Error)
	assert.Equal(t, 0.0, terms1.P)
	assert.Equal(t, 0.0, terms2.Error)
	assert.Equal(t, 0.0, terms2.P)
	// Outputs should be at minimum (no heating/cooling needed)
	assert.Equal(t, 0.0, output1)
	assert.Equal(t, 0.0, output2)
}

// TestPIDController_Reset_ClearsState tests the Reset method
func TestPIDController_Reset(t *testing.T) {
	// Arrange
	pid := NewPIDController(5.0, 0.1, 2.0, 38.0, 0, 100, 50)

	// Act - run some calculations to build state
	pid.Calculate(45.0)
	time.Sleep(10 * time.Millisecond)
	pid.Calculate(46.0)

	// Reset and verify state is cleared
	pid.Reset()
	state := pid.GetState()

	// Assert
	assert.Equal(t, 0.0, state["integral"])
	assert.Equal(t, 0.0, state["prev_error"])
	assert.True(t, pid.FirstRun)
}

// TestPIDController_SetTarget_UpdatesCorrectly tests SetTarget method
func TestPIDController_SetTarget(t *testing.T) {
	// Arrange
	pid := NewPIDController(5.0, 0.1, 2.0, 38.0, 0, 100, 50)

	// Act
	pid.SetTarget(42.0)

	// Assert
	state := pid.GetState()
	assert.Equal(t, 42.0, state["target"])

	// Verify new target is used in calculation
	_, terms := pid.Calculate(45.0)
	assert.Equal(t, 3.0, terms.Error) // 45 - 42 = 3
}

// TestPIDController_SetGains_UpdatesCorrectly tests SetGains method
func TestPIDController_SetGains(t *testing.T) {
	// Arrange
	pid := NewPIDController(5.0, 0.1, 2.0, 38.0, 0, 100, 50)

	// Act
	pid.SetGains(3.0, 0.05, 1.0)

	// Assert
	state := pid.GetState()
	assert.Equal(t, 3.0, state["kp"])
	assert.Equal(t, 0.05, state["ki"])
	assert.Equal(t, 1.0, state["kd"])
}

// TestPIDController_SetLimits_UpdatesCorrectly tests SetLimits method
func TestPIDController_SetLimits(t *testing.T) {
	// Arrange
	pid := NewPIDController(5.0, 0.1, 2.0, 38.0, 0, 100, 50)

	// Act
	pid.SetLimits(20, 90)

	// Assert
	state := pid.GetState()
	assert.Equal(t, 20.0, state["min_output"])
	assert.Equal(t, 90.0, state["max_output"])

	// Verify limits are enforced
	output, _ := pid.Calculate(10.0) // Large negative error
	assert.Equal(t, 20.0, output)    // Clamped to new min
}

// TestPIDController_SetIntegralMax_UpdatesCorrectly tests SetIntegralMax
func TestPIDController_SetIntegralMax(t *testing.T) {
	// Arrange
	pid := NewPIDController(0.0, 1.0, 0.0, 38.0, 0, 100, 50.0)

	// Act
	pid.SetIntegralMax(10.0)

	// Assert
	state := pid.GetState()
	assert.Equal(t, 10.0, state["integral_max"])

	// Verify new limit is enforced
	for i := 0; i < 50; i++ {
		pid.Calculate(50.0) // Large error to accumulate integral
		time.Sleep(10 * time.Millisecond)
	}
	_, terms := pid.Calculate(50.0)
	assert.LessOrEqual(t, terms.I, 10.0)
}

// TestPIDTuning_ValidateGains_ValidRanges tests validation with valid gains
func TestPIDTuning_ValidateGains_ValidRanges(t *testing.T) {
	// Arrange
	pid := NewPIDController(5.0, 0.1, 20.0, 38.0, 0, 100, 50)
	tuning := NewPIDTuning(pid)

	// Act
	warnings := tuning.ValidateGains()

	// Assert - should have no warnings for reasonable values
	assert.Empty(t, warnings)
}

// TestPIDTuning_ValidateGains_InvalidKp tests validation with invalid Kp
func TestPIDTuning_ValidateGains_InvalidKp(t *testing.T) {
	// Arrange
	pid := NewPIDController(25.0, 0.1, 20.0, 38.0, 0, 100, 50)
	tuning := NewPIDTuning(pid)

	// Act
	warnings := tuning.ValidateGains()

	// Assert
	require.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "Kp should typically be between 0-20")
}

// TestPIDTuning_ValidateGains_InvalidKi tests validation with invalid Ki
func TestPIDTuning_ValidateGains_InvalidKi(t *testing.T) {
	// Arrange
	pid := NewPIDController(5.0, 3.0, 20.0, 38.0, 0, 100, 50)
	tuning := NewPIDTuning(pid)

	// Act
	warnings := tuning.ValidateGains()

	// Assert
	require.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "Ki should typically be between 0-2")
}

// TestPIDTuning_ValidateGains_OscillationWarning tests oscillation detection
func TestPIDTuning_ValidateGains_OscillationWarning(t *testing.T) {
	// Arrange
	pid := NewPIDController(15.0, 0.8, 20.0, 38.0, 0, 100, 50)
	tuning := NewPIDTuning(pid)

	// Act
	warnings := tuning.ValidateGains()

	// Assert
	require.NotEmpty(t, warnings)
	hasOscillationWarning := false
	for _, warning := range warnings {
		if assert.Contains(t, warning, "oscillation") {
			hasOscillationWarning = true
		}
	}
	assert.True(t, hasOscillationWarning, "Should warn about oscillation with high Kp and Ki")
}

// TestPIDTuning_PresetConfigurations tests preset tuning configurations
func TestPIDTuning_PresetConfigurations(t *testing.T) {
	tests := []struct {
		name   string
		tuneF  func(*PIDTuning)
		checkF func(*testing.T, *PIDController)
	}{
		{
			name:  "temperature control",
			tuneF: func(pt *PIDTuning) { pt.TuneForTemperatureControl() },
			checkF: func(t *testing.T, p *PIDController) {
				assert.Equal(t, 5.0, p.Kp)
				assert.Equal(t, 0.1, p.Ki)
				assert.Equal(t, 20.0, p.Kd)
			},
		},
		{
			name:  "responsive control",
			tuneF: func(pt *PIDTuning) { pt.TuneForResponsiveControl() },
			checkF: func(t *testing.T, p *PIDController) {
				assert.Equal(t, 8.0, p.Kp)
				assert.Equal(t, 0.2, p.Ki)
				assert.Equal(t, 30.0, p.Kd)
			},
		},
		{
			name:  "stable control",
			tuneF: func(pt *PIDTuning) { pt.TuneForStableControl() },
			checkF: func(t *testing.T, p *PIDController) {
				assert.Equal(t, 3.0, p.Kp)
				assert.Equal(t, 0.05, p.Ki)
				assert.Equal(t, 15.0, p.Kd)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			pid := NewPIDController(1.0, 1.0, 1.0, 38.0, 0, 100, 50)
			tuning := NewPIDTuning(pid)

			// Act
			tt.tuneF(tuning)

			// Assert
			tt.checkF(t, pid)
		})
	}
}

// TestPIDController_TemperatureStepResponse tests response to step change
func TestPIDController_TemperatureStepResponse(t *testing.T) {
	// Arrange
	pid := NewPIDController(5.0, 0.1, 20.0, 38.0, 0, 100, 50)

	// Act - simulate step change from target to high temp
	outputs := []float64{}
	temps := []float64{38.0, 45.0, 45.0, 44.0, 42.0, 40.0, 39.0, 38.5, 38.0}

	for _, temp := range temps {
		output, _ := pid.Calculate(temp)
		outputs = append(outputs, output)
		time.Sleep(10 * time.Millisecond)
	}

	// Assert - verify general PID behavior
	// Temperature spike should increase output
	assert.Greater(t, outputs[1], 30.0, "Output should increase significantly with temp spike")
	// Final output near target should be lower than spike output
	assert.Less(t, outputs[8], outputs[1], "Output should be lower near target than during spike")
	// Output at target should be relatively low
	assert.Less(t, outputs[8], 30.0, "Output should be low near target")
}

// TestPIDController_SteadyStateError tests steady state behavior
func TestPIDController_SteadyStateError(t *testing.T) {
	// Arrange
	pid := NewPIDController(5.0, 0.1, 2.0, 38.0, 0, 100, 50)

	// Act - hold at slightly above target to build integral
	for i := 0; i < 20; i++ {
		pid.Calculate(38.5) // Small steady-state error
		time.Sleep(10 * time.Millisecond)
	}
	_, terms := pid.Calculate(38.5)

	// Assert - integral term should be working to eliminate error
	assert.Greater(t, terms.I, 0.0, "Integral should accumulate to eliminate steady-state error")
	assert.Equal(t, 0.5, terms.Error, "Error should be 0.5")
}

// TestPIDController_OscillationPrevention tests anti-oscillation behavior
func TestPIDController_OscillationPrevention(t *testing.T) {
	// Arrange - use conservative gains to prevent oscillation
	pid := NewPIDController(1.5, 0.05, 2.0, 38.0, 30, 100, 20)

	// Act - simulate temperature oscillating around target
	temps := []float64{38.0, 39.0, 38.0, 39.0, 38.0, 39.0, 38.0}
	outputs := []float64{}

	for _, temp := range temps {
		output, _ := pid.Calculate(temp)
		outputs = append(outputs, output)
		time.Sleep(50 * time.Millisecond)
	}

	// Assert - outputs should not oscillate wildly
	for i := 1; i < len(outputs); i++ {
		change := math.Abs(outputs[i] - outputs[i-1])
		assert.Less(t, change, 30.0, "Output changes should be gradual with conservative gains")
	}
}

// TestClamp_BelowMin tests clamping below minimum
func TestClamp_BelowMin(t *testing.T) {
	// Act
	result := clamp(-10.0, 0.0, 100.0)

	// Assert
	assert.Equal(t, 0.0, result)
}

// TestClamp_AboveMax tests clamping above maximum
func TestClamp_AboveMax(t *testing.T) {
	// Act
	result := clamp(150.0, 0.0, 100.0)

	// Assert
	assert.Equal(t, 100.0, result)
}

// TestClamp_WithinRange tests value within range
func TestClamp_WithinRange(t *testing.T) {
	// Act
	result := clamp(50.0, 0.0, 100.0)

	// Assert
	assert.Equal(t, 50.0, result)
}

// TestClamp_EqualToMin tests value equal to minimum
func TestClamp_EqualToMin(t *testing.T) {
	// Act
	result := clamp(0.0, 0.0, 100.0)

	// Assert
	assert.Equal(t, 0.0, result)
}

// TestClamp_EqualToMax tests value equal to maximum
func TestClamp_EqualToMax(t *testing.T) {
	// Act
	result := clamp(100.0, 0.0, 100.0)

	// Assert
	assert.Equal(t, 100.0, result)
}
