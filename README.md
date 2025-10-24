# Fan Controller

A modern Go-based fan controller for ASRock X570D4U-2L2T motherboards with PID control, Prometheus metrics, and Docker deployment.

## Features

- **PID Control**: Proportional-Integral-Derivative controller with anti-windup
- **Auto-Detection**: Automatically finds CPU temperature sensor and spinning disks
- **Prometheus Metrics**: Exposes metrics at `:9090/metrics` for monitoring
- **Docker Deployment**: Runs in Docker with hardware access
- **Safety Features**: Emergency overrides for high temperatures
- **Graceful Shutdown**: Sets fans to 100% on exit

## Hardware Compatibility

- **Motherboard**: ASRock X570D4U-2L2T (confirmed)
- **IPMI**: Uses `ipmitool` with raw command format `0x3a 0xd6`
- **Fans**: Controls 6 fan headers (FAN1-FAN6_1)
- **CPU Sensor**: k10temp (auto-detected)
- **Disk Sensors**: SATA and NVMe via `smartctl`

## Quick Start

### 1. Clone and Build

```bash
# Clone to server
git clone <repository-url> /mnt/ssd-storage/apps/fan-controller
cd /mnt/ssd-storage/apps/fan-controller

# Build Docker image
docker build -t fan-control:latest .
```

### 2. Configure

Create configuration file:

```bash
# Create config directory
mkdir -p $APPDIR/fan-control

# Copy and edit config
cp config.yaml $APPDIR/fan-control/config.yaml
nano $APPDIR/fan-control/config.yaml
```

Edit `$APPDIR/fan-control/config.yaml` to match your system:

```yaml
temperature:
  target_hdd: 38.0        # Target temperature for warmest disks
  max_hdd: 40.0           # Emergency override temperature
  max_cpu: 75.0           # CPU emergency temperature
  warmest_disks: 4        # Average temperature of N warmest disks

pid:
  kp: 5.0                 # Proportional gain
  ki: 0.1                 # Integral gain
  kd: 20.0                # Derivative gain
```

### 3. Test IPMI

Before deploying, test IPMI functionality:

```bash
# Test IPMI from fan-controller directory
docker run --rm --privileged \
  -v /dev/ipmi0:/dev/ipmi0 \
  -v /sys:/sys:ro \
  -v /dev:/dev:ro \
  fan-control:latest --test-ipmi
```

### 4. Deploy

The fan-control service has been added to your homelab-docker-configs/docker-compose.yml. Deploy it:

```bash
# From homelab-docker-configs directory
cd /path/to/homelab-docker-configs
docker-compose up -d fan-control
```

### 5. Monitor

Check logs and metrics:

```bash
# View logs
docker-compose logs -f fan-control

# Check metrics
curl http://localhost:9090/metrics

# Check health
curl http://localhost:9090/health
```

## Configuration

### Temperature Settings

```yaml
temperature:
  target_hdd: 38.0        # Target temperature (°C)
  max_hdd: 40.0           # Emergency override (°C)
  max_cpu: 75.0           # CPU emergency (°C)
  poll_interval: 30s      # Check interval
  warmest_disks: 4        # Average of N warmest disks
```

### Fan Settings

```yaml
fans:
  min_duty: 30            # Minimum fan duty (%)
  max_duty: 100           # Maximum fan duty (%)
  startup_duty: 50        # Initial duty on startup (%)
```

### PID Tuning

```yaml
pid:
  kp: 5.0                 # Proportional gain
  ki: 0.1                 # Integral gain
  kd: 20.0                # Derivative gain
  integral_max: 50.0      # Anti-windup limit
```

**PID Tuning Guide:**
- **Kp (Proportional)**: Higher = faster response, lower = more stable
- **Ki (Integral)**: Higher = eliminates steady-state error, lower = less oscillation
- **Kd (Derivative)**: Higher = reduces overshoot, lower = less noise sensitivity

**Starting Values:**
- Conservative: Kp=3.0, Ki=0.05, Kd=15.0
- Balanced: Kp=5.0, Ki=0.1, Kd=20.0 (default)
- Aggressive: Kp=8.0, Ki=0.2, Kd=30.0

### Disk Filtering

```yaml
disks:
  exclude_patterns:
    - "^loop"             # Loop devices
    - "^sr"               # CD-ROM
    - "^zram"             # Compressed RAM
    - "^zd"               # ZFS zvols
    - "^dm-"              # Device mapper
```

## CLI Options

```bash
# Test IPMI functionality
./fan-control --test-ipmi

# Dry run (no hardware changes)
./fan-control --dry-run

# Custom config file
./fan-control --config /path/to/config.yaml

# Override log level
./fan-control --log-level debug
```

## Prometheus Metrics

The controller exposes these metrics at `:9090/metrics`:

### Temperature Metrics
- `fan_controller_hdd_temperature_celsius{disk="sda"}` - Individual disk temperatures
- `fan_controller_hdd_temperature_max_celsius` - Highest disk temperature
- `fan_controller_hdd_temperature_avg_celsius` - Average of warmest disks
- `fan_controller_cpu_temperature_celsius` - CPU temperature

### Fan Metrics
- `fan_controller_fan_duty_percent` - Current fan duty cycle
- `fan_controller_fan_speed_rpm{fan="FAN1"}` - Individual fan speeds

### PID Metrics
- `fan_controller_pid_proportional` - P term
- `fan_controller_pid_integral` - I term
- `fan_controller_pid_derivative` - D term
- `fan_controller_pid_error_celsius` - Current error

### System Metrics
- `fan_controller_emergency_mode{reason="hdd_temp"}` - Emergency status
- `fan_controller_errors_total{type="ipmi"}` - Error counters
- `fan_controller_loop_duration_seconds` - Loop timing

## Telegraf Integration

Add to your `telegraf.conf`:

```toml
[[inputs.prometheus]]
  urls = ["http://fan-control:9090/metrics"]
  metric_version = 2
  interval = "60s"
  timeout = "5s"
  namedrop = ["go_*", "promhttp_*"]
```

Restart Telegraf after adding this configuration.

## Troubleshooting

### IPMI Commands Not Working

**Symptoms**: `ipmitool` commands fail, fans don't respond

**Solutions**:
1. Check IPMI device: `ls -la /dev/ipmi*`
2. Test IPMI: `docker-compose run --rm fan-control --test-ipmi`
3. Verify permissions: Container needs `privileged: true`
4. Check network: IPMI might be disabled in BIOS

### Can't Read Temperatures

**Symptoms**: Temperature reading errors, no disk/CPU temps

**Solutions**:
1. Check CPU sensor: `cat /sys/class/hwmon/hwmon*/name`
2. Check disk access: `smartctl -A /dev/sda`
3. Verify permissions: Container needs access to `/sys` and `/dev`
4. Check disk filtering: Adjust `exclude_patterns` in config

### Fans Not Responding

**Symptoms**: IPMI commands succeed but fans don't change speed

**Solutions**:
1. Check fan connections: Ensure fans are connected to FAN1-FAN6
2. Verify fan type: Some fans don't support PWM control
3. Check BIOS settings: Fan control might be disabled
4. Test manually: `ipmitool raw 0x3a 0xd6 0x32 0x32 0x32 0x32 0x32 0x32 0x64 0x64 0x64 0x64 0x64 0x64 0x64 0x64 0x64 0x64`

### PID Oscillating

**Symptoms**: Fan speeds constantly changing, temperature overshooting

**Solutions**:
1. Reduce Kp: Lower proportional gain
2. Reduce Kd: Lower derivative gain
3. Increase poll interval: Slower response
4. Check for noise: Ensure stable temperature readings

### Temperature Too High/Low

**Symptoms**: System runs too hot or too cold

**Solutions**:
1. Adjust target temperature: Change `target_hdd` in config
2. Tune PID gains: Increase Kp for faster response
3. Check fan curve: Verify fans can provide adequate cooling
4. Monitor metrics: Use Grafana to visualize temperature trends

### PID Oscillation (Fan Cycling)

**Symptoms**: 
- Fan duty cycle rapidly switching between low (30-60%) and high (100%) values
- Fan RPM following the same pattern
- Temperature readings relatively stable but fans overreacting
- Logs show frequent "EMERGENCY" mode entries

**Root Cause**: PID parameters too aggressive, causing overcorrection

**Solutions**:
1. **Reduce Derivative Gain (Kd)**: Most critical fix
   - Default: `kd: 20.0` → Recommended: `kd: 2.0`
   - Derivative gain amplifies rate of change, causing overcorrection

2. **Reduce Proportional Gain (Kp)**:
   - Default: `kp: 5.0` → Recommended: `kp: 1.5`
   - Less aggressive response to temperature errors

3. **Reduce Integral Gain (Ki)**:
   - Default: `ki: 0.1` → Recommended: `ki: 0.05`
   - Slower accumulation of error over time

4. **Increase Poll Interval**:
   - Default: `poll_interval: 30s` → Recommended: `poll_interval: 60s`
   - Gives system more time to respond to changes

5. **Raise Emergency Threshold**:
   - Default: `max_hdd: 40.0` → Recommended: `max_hdd: 42.0`
   - Reduces frequency of emergency mode triggers

**Final Working Configuration**:
```yaml
pid:
  kp: 1.5                 # Proportional gain (reduced from 5.0)
  ki: 0.05                # Integral gain (reduced from 0.1)
  kd: 2.0                 # Derivative gain (reduced from 20.0)
  integral_max: 20.0      # Anti-windup limit (reduced from 50.0)

temperature:
  max_hdd: 45.0           # Emergency threshold (increased from 40.0)
  poll_interval: 60s      # Poll interval (increased from 30s)

fans:
  min_duty: 60            # Minimum duty cycle (increased from 30%)
```

### Emergency Mode Oscillation (Most Common Issue)

**Symptoms**: 
- Regular sawtooth pattern: fans cycle between 40-60% and 100% every few minutes
- HDD temperatures consistently at 45-46°C
- Logs show frequent "EMERGENCY: hdd_temp" entries
- Fan duty cycle never stabilizes

**Root Cause**: Minimum duty cycle too low, allowing HDDs to heat up to emergency threshold

**Solution**:
1. **Increase min_duty**: Most critical fix
   - Default: `min_duty: 30` → Recommended: `min_duty: 60`
   - Prevents HDDs from heating up to emergency threshold
   - Maintains adequate baseline cooling

2. **Raise emergency threshold**:
   - Default: `max_hdd: 40.0` → Recommended: `max_hdd: 45.0`
   - Gives more headroom before emergency mode

3. **Verify target temperature is realistic**:
   - Default: `target_hdd: 38.0` (may be too low for some systems)
   - Consider raising to 40-42°C if system can't maintain 38°C

**Key Insight**: The ASRock X570D4U-2L2T has no dedicated CPU fan - all 6 fans are system fans controlled by HDD temperature. If min_duty is too low, HDDs heat up, trigger emergency mode, fans go to 100%, HDDs cool, fans drop to min_duty, cycle repeats.

## Safety Features

### Emergency Overrides
- **CPU > max_cpu**: Fans set to 100% immediately
- **Any disk > max_hdd**: Fans set to 100% immediately
- **5 consecutive IPMI failures**: Fans set to 100% immediately

### Graceful Shutdown
- **SIGTERM/SIGINT**: Fans set to 100% before exit
- **Container stop**: Docker sends SIGTERM, triggers safety mode
- **Health check failure**: Container restarts, fans reset to 100%

### Error Handling
- **Temperature failures**: Continues operation, logs errors
- **IPMI failures**: Retry logic, emergency mode after 5 failures
- **Configuration errors**: Validates on startup, fails fast

## Development

### Building Locally

```bash
# Install dependencies
go mod download

# Build binary
go build -o fan-control

# Test IPMI
./fan-control --test-ipmi

# Dry run
./fan-control --dry-run
```

### Docker Development

```bash
# Build image
docker build -t fan-control:dev .

# Run with custom config
docker run --rm -v /path/to/config.yaml:/config/config.yaml fan-control:dev

# Run in privileged mode for testing
docker run --rm --privileged -v /sys:/sys:ro -v /dev:/dev:ro fan-control:dev
```

## License

This project is licensed under the MIT License.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## Support

For issues and questions:
1. Check the troubleshooting section
2. Review the logs: `docker-compose logs fan-control`
3. Test IPMI manually: `ipmitool sensor | grep FAN`
4. Check metrics: `curl http://localhost:9090/metrics`
