# Fan Controller Deployment Guide

Complete deployment guide for the fan controller system including build, configuration, monitoring, and troubleshooting.

## Prerequisites

- **Hardware**: ASRock X570D4U-2L2T motherboard
- **OS**: Linux with Docker support
- **IPMI**: `ipmitool` installed and working
- **Monitoring**: Prometheus and Grafana (optional but recommended)

## Step 1: Build and Deploy

### 1.1 Clone Repository
```bash
# Clone to server
git clone <repository-url> /mnt/ssd-storage/apps/fan-controller
cd /mnt/ssd-storage/apps/fan-controller
```

### 1.2 Build Docker Image
```bash
# Build the fan controller image
docker build -t fan-control:latest .
```

### 1.3 Create Configuration
```bash
# Create config directory
mkdir -p $APPDIR/fan-control

# Copy default config
cp config.yaml $APPDIR/fan-control/config.yaml

# Edit configuration
nano $APPDIR/fan-control/config.yaml
```

## Step 2: Configuration

### 2.1 Basic Configuration
Edit `$APPDIR/fan-control/config.yaml`:

```yaml
server:
  metrics_port: 9090
  log_level: info

temperature:
  target_hdd: 38.0        # Target temp for warmest N disks (°C)
  max_hdd: 42.0           # Emergency override temp (°C)
  max_cpu: 75.0           # CPU emergency temp (°C)
  poll_interval: 60s      # How often to check temps and adjust fans
  warmest_disks: 4        # Average temp of this many warmest disks

fans:
  min_duty: 30            # Minimum fan duty cycle (%)
  max_duty: 100           # Maximum fan duty cycle (%)
  startup_duty: 50        # Initial fan duty on startup (%)

pid:
  kp: 1.5                 # Proportional gain (tuned for stability)
  ki: 0.05                # Integral gain (tuned for stability)
  kd: 2.0                 # Derivative gain (tuned for stability)
  integral_max: 20.0      # Anti-windup limit for integral term

disks:
  exclude_patterns:       # Regex patterns for disks to ignore
    - "^loop"             # Loop devices
    - "^sr"               # CD-ROM
    - "^zram"             # Compressed RAM
    - "^zd"               # ZFS zvols
    - "^dm-"              # Device mapper
```

### 2.2 Docker Compose Integration
Add to your `docker-compose.yml`:

```yaml
# Fan Controller - PID-based fan control for ASRock X570D4U-2L2T
fan-control:
  container_name: fan-control
  image: fan-control:latest
  privileged: true
  user: "0:0"  # Run as root for IPMI access
  deploy:
    restart_policy:
      condition: on-failure
      delay: 5s
      max_attempts: 3
      window: 10s
  networks:
    - t2_proxy
  devices:
    - /dev/ipmi0:/dev/ipmi0
  volumes:
    - $APPDIR/fan-control/config.yaml:/config/config.yaml:ro
    - /sys:/sys:ro
    - /dev:/dev:ro
    - /etc/timezone:/etc/timezone:ro
    - /etc/localtime:/etc/localtime:ro
  environment:
    TZ: $TZ
  healthcheck:
    test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:9090/health"]
    interval: 30s
    timeout: 10s
    retries: 3
    start_period: 40s
  labels:
    - "managed=true"

# Prometheus - Metrics collection for fan-control
prometheus:
  image: prom/prometheus:latest
  container_name: prometheus
  networks:
    - t2_proxy
  volumes:
    - $APPDIR/prometheus:/prometheus
    - $APPDIR/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
  ports:
    - "9091:9090"
  command:
    - '--storage.tsdb.path=/prometheus'
    - '--web.console.libraries=/etc/prometheus/console_libraries'
    - '--web.console.templates=/etc/prometheus/consoles'
    - '--config.file=/etc/prometheus/prometheus.yml'
    - '--web.enable-lifecycle'
  labels:
    - "managed=true"
```

### 2.3 Prometheus Configuration
Create `$APPDIR/prometheus/prometheus.yml`:

```yaml
global:
  scrape_interval: 30s

scrape_configs:
  - job_name: 'fan-control'
    static_configs:
      - targets: ['fan-control:9090']
```

## Step 3: Start Services

### 3.1 Start Fan Controller
```bash
# Start the fan controller
docker compose up -d fan-control

# Check status
docker compose ps fan-control
docker logs fan-control
```

### 3.2 Start Prometheus (if not already running)
```bash
# Start Prometheus
docker compose up -d prometheus

# Check metrics
curl http://localhost:9091/api/v1/query?query=fan_controller_cpu_temperature_celsius
```

## Step 4: Grafana Dashboard

### 4.1 Import Dashboard
1. Open Grafana (usually `http://your-server:3000`)
2. Go to **Import** (the + icon)
3. Click **"Upload JSON file"**
4. Select: `grafana-dashboard.json` from this repository
5. Configure:
   - **Name**: "Fan Controller Dashboard"
   - **Prometheus Data Source**: Select your Prometheus datasource
   - Click **"Import"**

### 4.2 Dashboard Features
The dashboard includes:
- **HDD Temperature** monitoring
- **CPU Temperature** monitoring  
- **Fan Duty Cycle** monitoring
- **Fan RPM** monitoring (all fans with labels)
- **5-second refresh rate**
- **1-hour default time range**

## Step 5: Verification and Tuning

### 5.1 Check Metrics
```bash
# Check if metrics are being exposed
curl http://fan-control:9090/metrics | grep fan_controller

# Check Prometheus is scraping
curl http://prometheus:9090/api/v1/targets
```

### 5.2 Monitor Performance
Watch the Grafana dashboard for:
- **Stable temperature readings**
- **Smooth fan duty cycle changes**
- **No oscillation or rapid cycling**

### 5.3 PID Tuning (if needed)

**If experiencing oscillation:**
1. **Reduce Kd (derivative)**: Most critical
   - Change `kd: 2.0` to `kd: 1.0` or lower
2. **Reduce Kp (proportional)**: Less aggressive
   - Change `kp: 1.5` to `kp: 1.0`
3. **Increase poll interval**: Slower response
   - Change `poll_interval: 60s` to `poll_interval: 120s`

**If response too slow:**
1. **Increase Kp**: More aggressive
   - Change `kp: 1.5` to `kp: 2.0`
2. **Decrease poll interval**: Faster response
   - Change `poll_interval: 60s` to `poll_interval: 30s`

## Step 6: Troubleshooting

### 6.1 Common Issues

**No metrics appearing:**
```bash
# Check container is running
docker ps | grep fan-control

# Check logs
docker logs fan-control

# Test IPMI manually
ipmitool raw 0x3a 0xd6 0x32 0x00 0x00 0x00 0x00 0x00 0x00
```

**Fan oscillation:**
- See PID tuning section above
- Check logs for "EMERGENCY" entries
- Verify temperature readings are stable

**High CPU usage:**
- Increase `poll_interval` in config
- Check for excessive logging
- Verify no other processes are using IPMI

### 6.2 Log Analysis
```bash
# Monitor real-time logs
docker logs -f fan-control

# Check for errors
docker logs fan-control 2>&1 | grep -i error

# Check PID behavior
docker logs fan-control | grep "Status:"
```

### 6.3 Emergency Procedures
```bash
# Set fans to 100% manually (emergency)
ipmitool raw 0x3a 0xd6 0x64 0x00 0x00 0x00 0x00 0x00 0x00

# Stop fan controller
docker stop fan-control

# Restart with safe settings
docker start fan-control
```

## Step 7: Maintenance

### 7.1 Regular Monitoring
- Check Grafana dashboard weekly
- Monitor temperature trends
- Verify fan performance

### 7.2 Configuration Updates
```bash
# Update config
nano $APPDIR/fan-control/config.yaml

# Restart service
docker restart fan-control

# Verify changes
docker logs fan-control --tail 10
```

### 7.3 Backup Configuration
```bash
# Backup working config
cp $APPDIR/fan-control/config.yaml $APPDIR/fan-control/config.yaml.backup

# Backup Prometheus data
docker exec prometheus tar -czf /prometheus/backup-$(date +%Y%m%d).tar.gz /prometheus
```

## Final Notes

- **Safety First**: The system has multiple safety overrides
- **Monitor Initially**: Watch for 24-48 hours after deployment
- **Tune Gradually**: Make small PID adjustments, test, then adjust again
- **Document Changes**: Keep track of what works for your specific hardware

The fan controller is now fully deployed and monitoring your system temperatures with intelligent PID control!
