# 🤖 Blackbox Virtual Robot Fleet

A sophisticated fleet simulator for the **Blackbox** mini Kafka pub/sub broker featuring 8 warehouse robots with synthetic sensor data, failure patterns, and real-time monitoring dashboard.

## 🎯 Overview

This module simulates a complete warehouse robotics fleet with:
- **8 Virtual Robots** across 4 different roles
- **32+ Sensor Types** with realistic synthetic data
- **4 Failure Patterns** with deterministic timing
- **Real-time Dashboard** via React + WebSocket
- **Lightweight Relay Server** for local data streaming
- **One-click Startup** for hackathon demos

## 🤖 Fleet Composition

### NAVIGATOR (Path Planning) - 2 robots
- **WH-NAV-01 "Pathfinder Alpha"** → Stable
- **WH-NAV-02 "Pathfinder Beta"** → Collision Cascade (T+80s)

**Job:** Navigate warehouse shelves with precision
**Sensors:** proximity, battery, motor_temp, speed, vibration, obstacle, orientation, path_deviation

### CARRIER (Heavy Transport) - 2 robots
- **WH-CAR-01 "HeavyLift One"** → Degrading
- **WH-CAR-02 "HeavyLift Two"** → Thermal Runaway (T+140s)

**Job:** Transport packages across warehouse
**Sensors:** proximity, battery, motor_temp, speed, vibration, obstacle, load_weight, axle_stress

### SCANNER (Inventory) - 2 robots
- **WH-SCN-01 "Sentinel Prime"** → Stable
- **WH-SCN-02 "Sentinel Echo"** → Mechanical Failure (T+260s)

**Job:** Scan and verify shelf inventory
**Sensors:** proximity, battery, motor_temp, speed, vibration, obstacle, scan_accuracy, arm_angle

### CHARGE RUNNER (Battery Management) - 2 robots
- **WH-RUN-01 "DockDasher One"** → Degrading
- **WH-RUN-02 "DockDasher Two"** → Battery Death (T+200s)

**Job:** Return low-battery robots to charging dock
**Sensors:** proximity, battery, motor_temp, speed, vibration, obstacle, charge_rate, eta_to_dock

## 📊 Failure Patterns

Each robot failure is deterministic and repeatable, demonstrating different causal chains:

### Pattern 1: Collision Cascade (WH-NAV-02)
- **T+50s:** Vibration increases rapidly
- **T+55s:** Path deviation drifts from planned route
- **T+60s:** Obstacle detection triggered
- **T+65s:** Proximity drops (collision imminent)
- **T+75s:** Speed drops to 0
- **T+80s:** **CRASH** - Complete system shutdown
- **Story:** "Navigation failure led to collision"

### Pattern 2: Thermal Runaway (WH-CAR-02)
- **T+110s:** Load weight exceeds capacity (overloaded)
- **T+115s:** Motor temperature begins rising
- **T+125s:** Axle stress increases sharply
- **T+130s:** Vibration spikes (mechanical stress)
- **T+135s:** Speed reduces (motor struggling)
- **T+140s:** **CRASH** - Thermal shutdown at 78°C
- **Story:** "Overloaded carrier caused thermal shutdown"

### Pattern 3: Battery Death (WH-RUN-02)
- **T+170s:** Battery depletes toward critical level
- **T+180s:** Speed reduction begins
- **T+185s:** Vibration drops (weakening motors)
- **T+190s:** Charge rate = 0 (too far from dock)
- **T+195s:** Speed = 0 (robot stalls)
- **T+200s:** **CRASH** - Complete battery failure
- **Story:** "Failed to reach dock in time"

### Pattern 4: Mechanical Failure (WH-SCN-02)
- **T+230s:** Scanner arm drifts erratically
- **T+235s:** Scan accuracy drops below 60%
- **T+240s:** Vibration spikes (mechanical grinding)
- **T+245s:** Robot tilts (balance issue)
- **T+250s:** Proximity readings become erratic
- **T+255s:** Speed = 0 (safety stop)
- **T+260s:** **CRASH** - Full mechanical shutdown
- **Story:** "Scanner arm failure caused system halt"

## 🎨 Live Dashboard

The React frontend displays:
- **Real-time Status Cards** for each robot (8-robot grid layout)
- **Color-coded Health Indicators**
  - 🟢 Green (ONLINE) - Normal operation
  - 🟡 Yellow (ALERT) - Warning thresholds exceeded
  - 🔴 Red (OFFLINE) - Complete failure
- **Key Metrics** specific to each robot role
- **Elapsed Time** counter
- **Critical Alerts** with pulsing animations

### Threshold Monitoring
```
Battery:          Critical < 15%, Warning < 30%
Motor Temp:       Critical > 75°C, Warning > 65°C
Proximity:        Critical < 20cm, Warning < 50cm
Vibration:        Critical > 80, Warning > 60
Scan Accuracy:    Critical < 50%, Warning < 75%
Axle Stress:      Critical > 85, Warning > 70
```

## 📊 Data Publishing

Each robot publishes sensor data every **300ms** to the local relay server:

```
HTTP POST to http://localhost:9000/publish
Payload: {"nodeId": "WH-NAV-01", "topic": "WH-NAV-01.battery", "value": 85.3}
```

The relay server broadcasts all messages via WebSocket (`ws://localhost:9000/ws`) to connected frontends.

This design keeps the fleet simulator **completely self-contained** — no broker modifications needed!

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Node.js 14+
- npm

### One-Click Startup (Linux/macOS)
```bash
cd v-robot-fleet
chmod +x demo.sh
./demo.sh
```

### One-Click Startup (Windows)
```bash
cd v-robot-fleet
demo.bat
```

### Manual Startup

**Terminal 1 - Start Relay Server:**
```bash
cd v-robot-fleet/relay
go run relay.go
```

**Terminal 2 - Start Simulator:**
```bash
cd v-robot-fleet/simulator
go run main.go
```

**Terminal 3 - Start Frontend:**
```bash
cd v-robot-fleet/frontend
npm install --legacy-peer-deps
npm start
```

Then open: **http://localhost:3000**

## 📁 Architecture

```
┌─────────────────────────────────────────────────┐
│           VIRTUAL ROBOT FLEET                   │
├─────────────────────────────────────────────────┤
│                                                 │
│  ┌──────────────────────────────────────────┐   │
│  │  8 Virtual Robots                        │   │
│  │  (Simulator - Go)                        │   │
│  └──────────────────────────────────────────┘   │
│              ↓ HTTP POST                        │
│  ┌──────────────────────────────────────────┐   │
│  │  Relay Server (Go)                       │   │
│  │  Port 9000: WebSocket + HTTP             │   │
│  └──────────────────────────────────────────┘   │
│          ↓ WebSocket (:9000/ws)                │
│  ┌──────────────────────────────────────────┐   │
│  │  React Dashboard                         │   │
│  │  Port 3000                               │   │
│  └──────────────────────────────────────────┘   │
│                                                 │
└─────────────────────────────────────────────────┘
```

**Self-Contained:** The fleet simulator is completely independent. No broker modifications needed!

## 🎬 Demo Timeline

```
T+0s    → All 8 robots online and stable
T+50s   → WH-NAV-02 degradation visible (vibration rising)
T+80s   → WH-NAV-02 CRASHES (collision cascade)
T+110s  → WH-CAR-02 degradation begins (overloaded)
T+140s  → WH-CAR-02 CRASHES (thermal runaway)
T+170s  → WH-RUN-02 degradation visible (battery dropping)
T+200s  → WH-RUN-02 CRASHES (battery dead)
T+230s  → WH-SCN-02 degradation begins (arm drift)
T+260s  → WH-SCN-02 CRASHES (mechanical failure)
```

**Demo Duration:** ~5 minutes of real-time action

## 📁 Directory Structure

```
v-robot-fleet/
├── relay/
│   ├── relay.go         # WebSocket relay server
│   └── go.mod           # Go module file
├── simulator/
│   ├── main.go          # Robot fleet simulator
│   └── go.mod           # Go module file
├── frontend/
│   ├── public/
│   │   └── index.html   # React entry point
│   ├── src/
│   │   ├── App.js       # Main dashboard component
│   │   ├── App.css      # Dashboard styling
│   │   ├── index.js     # React boot
│   │   └── index.css    # Global styles
│   ├── .gitignore       # Ignore node_modules
│   └── package.json     # Node dependencies
├── demo.sh              # Linux/macOS startup
├── demo.bat             # Windows startup
└── README.md            # This file
```

## 🔧 Customization

### Add New Robots
Edit `simulator/main.go` and add to the `createFleet()` function.

### Modify Failure Timing
Update the `CrashTime` field in robot definitions or edit the pattern functions.

### Change Dashboard Thresholds
Edit `CRITICAL_THRESHOLDS` in `frontend/src/App.js`.

### Adjust Sensor Publishing Rate
Change the ticker interval in `publishData()` (default: 300ms).

## 🎯 Hackathon Tips

1. **Visual Impact:** The dashboard automatically color-codes failures
2. **Live Demo:** All timing is deterministic - same behavior every run
3. **Multiple Failures:** Shows different causal chains in parallel
4. **Real Metrics:** All values are physically plausible
5. **Narrative:** Each failure tells a story about what went wrong

## 🐛 Troubleshooting

**Dashboard won't connect:**
- Ensure relay is running on `localhost:9000`
- Check that simulator is publishing data
- Verify WebSocket port 9000 is open

**Robots not appearing:**
- Check simulator console for output messages
- Verify relay started before simulator
- Look for "client registered" messages in relay

**No data flowing:**
- Confirm all 3 services started successfully
- Check all three terminal outputs for error messages
- Stop and restart all services

**Port already in use:**
- Port 3000 (frontend): `lsof -i :3000` and kill the process
- Port 9000 (relay): `lsof -i :9000` and kill the process

## 📝 Notes

- This is a **demonstration-grade** simulator optimized for visual impact
- All failure patterns are **deterministic** (same every run)
- Data is **synthetic** but physically realistic
- The broker currently stores all messages - check SQLite for persistence

## 🎓 Educational Value

This simulator demonstrates:
- Distributed system monitoring
- Real-time anomaly detection
- Predictable failure analysis
- Time-series data visualization
- Multi-robot coordination challenges

Perfect for explaining intelligent warehouse systems to hackathon judges!
