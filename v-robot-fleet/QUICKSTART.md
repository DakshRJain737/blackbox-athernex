# 🚀 QUICK START - Fleet Simulator Demo

## For Judges: 30-Second Launch

### Option 1: One Command (Linux/macOS)
```bash
cd v-robot-fleet && chmod +x demo.sh && ./demo.sh
```

### Option 2: One Command (Windows)
```bash
cd v-robot-fleet && demo.bat
```

**Result:** Three windows will open
1. **Relay Server** (yellow) - Broadcasting robot data
2. **Simulator** (green) - 8 virtual robots generating data
3. **Dashboard** (browser) - Opens automatically at http://localhost:3000

---

## What You'll See

### ⏱️ Timeline (5-minute demo)

```
T+0s    → All 8 robots online (green cards)
T+50s   → WH-NAV-02 starts degrading (yellow card)
T+80s   → WH-NAV-02 CRASHES (red card, all sensors → 0)
T+110s  → WH-CAR-02 starts degrading
T+140s  → WH-CAR-02 CRASHES (thermal runaway)
T+170s  → WH-RUN-02 starts degrading
T+200s  → WH-RUN-02 CRASHES (battery death)
T+230s  → WH-SCN-02 starts degrading
T+260s  → WH-SCN-02 CRASHES (mechanical failure)
```

### 📊 Dashboard Features

- **8 Robot Cards** in 2x4 grid
- **Color Coding:** 🟢 Online → 🟡 Alert → 🔴 Offline
- **Key Metrics** for each robot role
- **Live Elapsed Time** counter
- **Pulsing Alerts** when critical thresholds exceeded
- **Responsive Design** - Works on phones too

---

## The Architecture

```
Simulator (8 robots)
     ↓ HTTP POST every 300ms
Relay Server (Port 9000)
     ↓ WebSocket Broadcast
React Dashboard (Port 3000)
     ↓ Displays Real-Time Data
```

**Why this design?**
- ✅ No broker modifications needed
- ✅ Completely self-contained in v-robot-fleet/
- ✅ Fast, lightweight, deterministic
- ✅ Perfect for hackathon demos

---

## Robot Fleet

### Role 1: NAVIGATORS (Long-distance path planning)
- **WH-NAV-01** "Pathfinder Alpha" - Stable
- **WH-NAV-02** "Pathfinder Beta" - Crashes at T+80s (collision cascade)

### Role 2: CARRIERS (Heavy package transport)
- **WH-CAR-01** "HeavyLift One" - Degrading slowly
- **WH-CAR-02** "HeavyLift Two" - Crashes at T+140s (thermal runaway)

### Role 3: SCANNERS (Inventory verification)
- **WH-SCN-01** "Sentinel Prime" - Stable
- **WH-SCN-02** "Sentinel Echo" - Crashes at T+260s (mechanical failure)

### Role 4: CHARGE RUNNERS (Battery management)
- **WH-RUN-01** "DockDasher One" - Degrading slowly
- **WH-RUN-02** "DockDasher Two" - Crashes at T+200s (battery death)

---

## Sensor Data (32+ total sensors)

Each robot publishes:
- **Proximity** - Distance to obstacles (0-200cm)
- **Battery** - Power level (0-100%)
- **Motor Temp** - Temperature (20-80°C) ⚠️ Critical > 75°C
- **Speed** - Velocity (0-60 cm/s)
- **Vibration** - Motor vibration (0-100)
- **Obstacle** - Collision detection (0 or 1)

Plus role-specific sensors:
- Navigators: orientation, path deviation
- Carriers: load weight, axle stress
- Scanners: scan accuracy, arm angle
- Runners: charge rate, ETA to dock

---

## Failure Patterns (Deterministic)

### Pattern 1: Collision Cascade (WH-NAV-02)
- Navigation failure leads to obstacle collision
- Vibration ↑ → Path drift ↑ → Obstacle detected → Proximity ↓ → Speed ↓ → Crash

### Pattern 2: Thermal Runaway (WH-CAR-02)
- Overloaded carrier exceeds weight capacity
- Load ↑ → Temp ↑ → Stress ↑ → Vibration ↑ → Speed ↓ → Shutdown

### Pattern 3: Battery Death (WH-RUN-02)
- Started with 12% battery, couldn't reach dock
- Battery ↓ → Speed ↓ → Vibration ↓ → Stalled → Dead

### Pattern 4: Mechanical Failure (WH-SCN-02)
- Scanner arm bearing failure
- Arm drift ↑ → Accuracy ↓ → Vibration ↑ → Tilt ↑ → Proximity erratic → Stop

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Port 3000 in use | `lsof -i :3000` then kill process |
| Port 9000 in use | `lsof -i :9000` then kill process |
| Relay won't start | Ensure Go 1.21+ is installed |
| npm install fails | Use `--legacy-peer-deps` flag |
| Dashboard blank | Check browser console (F12) for errors |
| No robot data | Wait 2 seconds, reload browser |

---

## Pro Tips for Judges

1. **Show the timeline** - Point out exact times when each robot fails
2. **Explain the sensors** - Hover over metrics to see real-time values
3. **Discuss failures** - Each pattern tells a story about robot failure modes
4. **Mention data** - All values are synthetic but physically realistic
5. **Integration potential** - Could feed into ML anomaly detection engines

---

## Next Steps (After Hackathon)

The simulator can easily integrate with:
- 🔌 Real hardware via serial ports
- 📊 Database persistence (SQLite)
- 🤖 ML anomaly detection
- 📈 Historical analysis & replay
- 🎛️ Control commands for robots
- 📱 Mobile dashboard app

**The demo proves the concept works end-to-end!**
