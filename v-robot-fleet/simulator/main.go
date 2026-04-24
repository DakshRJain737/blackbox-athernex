package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// Message for publishing to relay HTTP endpoint
type Message struct {
	NodeID string  `json:"nodeId"`
	Topic  string  `json:"topic"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit,omitempty"`
}

// Robot represents a warehouse robot
type Robot struct {
	ID             string
	Role           string
	Name           string
	Status         string
	Sensors        map[string]float64
	Initial        map[string]float64
	BehaviorType   string
	CrashTime      time.Duration
	StartTime      time.Time
	RelayURL       string
	CrashPublished bool
	mu             sync.Mutex
	httpClient     *http.Client
}

// SensorConfig defines sensor bounds
type SensorConfig struct {
	Min   float64
	Max   float64
	Noise float64
	Unit  string
}

var sensorConfigs = map[string]SensorConfig{
	"proximity":      {Min: 0, Max: 200, Noise: 0.02, Unit: "cm"},
	"battery":        {Min: 0, Max: 100, Noise: 0.02, Unit: "%"},
	"motor_temp":     {Min: 20, Max: 90, Noise: 0.02, Unit: "C"},
	"speed":          {Min: 0, Max: 60, Noise: 0.02, Unit: "cm/s"},
	"vibration":      {Min: 0, Max: 120, Noise: 0.02, Unit: "Hz"},
	"obstacle":       {Min: 0, Max: 1, Noise: 0, Unit: "flag"},
	"orientation":    {Min: -180, Max: 180, Noise: 0.02, Unit: "deg"},
	"path_deviation": {Min: 0, Max: 30, Noise: 0.02, Unit: "cm"},
	"load_weight":    {Min: 0, Max: 40, Noise: 0.02, Unit: "kg"},
	"axle_stress":    {Min: 0, Max: 100, Noise: 0.02, Unit: "%"},
	"scan_accuracy":  {Min: 0, Max: 100, Noise: 0.02, Unit: "%"},
	"arm_angle":      {Min: 0, Max: 120, Noise: 0.02, Unit: "deg"},
	"charge_rate":    {Min: 0, Max: 100, Noise: 0.02, Unit: "%"},
	"eta_to_dock":    {Min: 0, Max: 300, Noise: 0.02, Unit: "s"},
}

func main() {
	robots := createFleet()
	relayURL := "http://localhost:9000"
	startTime := time.Now()

	fmt.Println("\n🤖 Fleet Simulator Starting...")
	fmt.Printf("   Relay: %s\n", relayURL)
	fmt.Printf("   Dashboard: http://localhost:3000\n\n")

	for _, robot := range robots {
		robot.StartTime = startTime
		robot.RelayURL = relayURL
		robot.httpClient = &http.Client{Timeout: 5 * time.Second}
		robot.Initial = cloneSensors(robot.Sensors)
		fmt.Printf("   ✓ %s (%s)\n", robot.ID, robot.Name)
		go robot.publishData()
		time.Sleep(50 * time.Millisecond)
	}

	go startControlServer(robots)
	fmt.Println("\n✨ All robots online and streaming sensor data...")
	select {}
}

func createFleet() []*Robot {
	return []*Robot{
		// NAVIGATOR
		{ID: "WH-NAV-01", Role: "Navigator", Name: "Pathfinder Alpha", Status: "ONLINE", BehaviorType: "stable",
			Sensors: map[string]float64{"proximity": 100, "battery": 85, "motor_temp": 35, "speed": 30, "vibration": 10, "obstacle": 0, "orientation": 45, "path_deviation": 0}},
		{ID: "WH-NAV-02", Role: "Navigator", Name: "Pathfinder Beta", Status: "ONLINE", BehaviorType: "crash_pattern_1", CrashTime: 80 * time.Second,
			Sensors: map[string]float64{"proximity": 100, "battery": 87, "motor_temp": 34, "speed": 32, "vibration": 8, "obstacle": 0, "orientation": 48, "path_deviation": 0}},
		// CARRIER
		{ID: "WH-CAR-01", Role: "Carrier", Name: "HeavyLift One", Status: "ONLINE", BehaviorType: "degrading",
			Sensors: map[string]float64{"proximity": 120, "battery": 81, "motor_temp": 42, "speed": 25, "vibration": 15, "obstacle": 0, "load_weight": 18, "axle_stress": 35}},
		{ID: "WH-CAR-02", Role: "Carrier", Name: "HeavyLift Two", Status: "ONLINE", BehaviorType: "crash_pattern_2", CrashTime: 140 * time.Second,
			Sensors: map[string]float64{"proximity": 118, "battery": 79, "motor_temp": 41, "speed": 24, "vibration": 14, "obstacle": 0, "load_weight": 17, "axle_stress": 33}},
		// SCANNER
		{ID: "WH-SCN-01", Role: "Scanner", Name: "Sentinel Prime", Status: "ONLINE", BehaviorType: "stable",
			Sensors: map[string]float64{"proximity": 30, "battery": 92, "motor_temp": 28, "speed": 8, "vibration": 5, "obstacle": 0, "scan_accuracy": 98, "arm_angle": 45}},
		{ID: "WH-SCN-02", Role: "Scanner", Name: "Sentinel Echo", Status: "ONLINE", BehaviorType: "crash_pattern_4", CrashTime: 260 * time.Second,
			Sensors: map[string]float64{"proximity": 32, "battery": 90, "motor_temp": 29, "speed": 7, "vibration": 4, "obstacle": 0, "scan_accuracy": 97, "arm_angle": 42}},
		// CHARGE RUNNER
		{ID: "WH-RUN-01", Role: "ChargeRunner", Name: "DockDasher One", Status: "ONLINE", BehaviorType: "degrading",
			Sensors: map[string]float64{"proximity": 140, "battery": 15, "motor_temp": 38, "speed": 45, "vibration": 20, "obstacle": 0, "charge_rate": 0, "eta_to_dock": 120}},
		{ID: "WH-RUN-02", Role: "ChargeRunner", Name: "DockDasher Two", Status: "ONLINE", BehaviorType: "crash_pattern_3", CrashTime: 200 * time.Second,
			Sensors: map[string]float64{"proximity": 135, "battery": 12, "motor_temp": 37, "speed": 44, "vibration": 19, "obstacle": 0, "charge_rate": 0, "eta_to_dock": 118}},
	}
}

func (r *Robot) publishData() {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		elapsed := time.Since(r.StartTime)
		r.updateSensors(elapsed)

		if r.Status == "OFFLINE" {
			if !r.CrashPublished {
				r.publishAllSensors()
				r.CrashPublished = true
			}
			continue
		}

		r.publishAllSensors()
	}
}

func (r *Robot) publishAllSensors() {
	for sensorName, value := range r.Sensors {
		cfg := sensorConfigs[sensorName]
		msg := Message{
			NodeID: r.ID,
			Topic:  sensorName,
			Value:  value,
			Unit:   cfg.Unit,
		}
		r.publishMessage(msg)
	}
}

func (r *Robot) publishMessage(msg Message) {
	r.mu.Lock()
	defer r.mu.Unlock()

	jsonData, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("%s marshal error: %v\n", r.ID, err)
		return
	}

	resp, err := r.httpClient.Post(r.RelayURL+"/publish", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("%s publish error: %v\n", r.ID, err)
		return
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
}

func (r *Robot) updateSensors(elapsed time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch r.BehaviorType {
	case "stable":
		r.updateStableRobot(elapsed)
	case "degrading":
		r.updateDegradingRobot(elapsed)
	case "crash_pattern_1":
		r.updateCrashPattern1(elapsed)
	case "crash_pattern_2":
		r.updateCrashPattern2(elapsed)
	case "crash_pattern_3":
		r.updateCrashPattern3(elapsed)
	case "crash_pattern_4":
		r.updateCrashPattern4(elapsed)
	}
}

func (r *Robot) updateStableRobot(elapsed time.Duration) {
	for sensorName := range r.Sensors {
		if sensorName == "obstacle" {
			continue
		}
		drift := math.Sin(float64(elapsed.Seconds())*0.5) * 0.5
		noise := randNormal() * 0.02
		r.Sensors[sensorName] += drift + noise
		r.clampSensor(sensorName)
	}
}

func (r *Robot) updateDegradingRobot(elapsed time.Duration) {
	degradationRate := 0.1 / float64(60*5)
	multiplier := 1.0 - (float64(elapsed.Seconds()) * degradationRate)

	for sensorName, value := range r.Sensors {
		if sensorName != "obstacle" {
			r.Sensors[sensorName] = value * multiplier
			r.clampSensor(sensorName)
		}
	}
}

func (r *Robot) updateCrashPattern1(elapsed time.Duration) {
	sec := elapsed.Seconds()
	if sec < 50 {
		r.updateStableRobot(elapsed)
		return
	}
	if sec >= 50 && sec < 55 {
		r.Sensors["vibration"] += 2 * (sec - 50)
	} else if sec >= 55 && sec < 60 {
		r.Sensors["path_deviation"] += 0.3 * (sec - 55)
	} else if sec >= 60 && sec < 65 {
		r.Sensors["obstacle"] = 1
	} else if sec >= 65 && sec < 75 {
		r.Sensors["proximity"] = math.Max(0, r.Sensors["proximity"]-10*(sec-65))
	} else if sec >= 75 && sec < 80 {
		r.Sensors["speed"] = 0
	} else if sec >= 80 {
		r.crashRobot()
	}
	r.clampAll()
}

func (r *Robot) updateCrashPattern2(elapsed time.Duration) {
	sec := elapsed.Seconds()
	if sec < 110 {
		r.updateStableRobot(elapsed)
		return
	}
	if sec >= 110 && sec < 115 {
		r.Sensors["load_weight"] = 24
	} else if sec >= 115 && sec < 125 {
		r.Sensors["motor_temp"] += 0.5 * (sec - 115)
	} else if sec >= 125 && sec < 130 {
		r.Sensors["axle_stress"] += 1.5 * (sec - 125)
	} else if sec >= 130 && sec < 135 {
		r.Sensors["vibration"] += 8 * (sec - 130)
	} else if sec >= 135 && sec < 140 {
		r.Sensors["speed"] = math.Max(0, r.Sensors["speed"]-2*(sec-135))
	} else if sec >= 140 {
		r.crashRobot()
	}
	r.clampAll()
}

func (r *Robot) updateCrashPattern3(elapsed time.Duration) {
	sec := elapsed.Seconds()
	if sec < 170 {
		r.updateStableRobot(elapsed)
		return
	}
	if sec >= 170 && sec < 180 {
		r.Sensors["battery"] = math.Max(0, 12-4*(sec-170)/10)
		r.Sensors["eta_to_dock"] += 5
	} else if sec >= 180 && sec < 185 {
		r.Sensors["speed"] = math.Max(0, r.Sensors["speed"]-2*(sec-180))
	} else if sec >= 185 && sec < 190 {
		r.Sensors["vibration"] = math.Max(0, r.Sensors["vibration"]-2*(sec-185))
	} else if sec >= 190 && sec < 195 {
		r.Sensors["charge_rate"] = 0
	} else if sec >= 195 && sec < 200 {
		r.Sensors["speed"] = 0
	} else if sec >= 200 {
		r.crashRobot()
	}
	r.clampAll()
}

func (r *Robot) updateCrashPattern4(elapsed time.Duration) {
	sec := elapsed.Seconds()
	if sec < 230 {
		r.updateStableRobot(elapsed)
		return
	}
	if sec >= 230 && sec < 235 {
		r.Sensors["arm_angle"] += rand.Float64()*10 - 5
	} else if sec >= 235 && sec < 240 {
		r.Sensors["scan_accuracy"] = math.Max(0, 97-8*(sec-235)/5)
	} else if sec >= 240 && sec < 245 {
		r.Sensors["vibration"] += 15 * (sec - 240)
	} else if sec >= 245 && sec < 250 {
		r.Sensors["orientation"] += rand.Float64()*20 - 10
	} else if sec >= 250 && sec < 255 {
		r.Sensors["proximity"] += rand.Float64()*20 - 10
	} else if sec >= 255 && sec < 260 {
		r.Sensors["speed"] = 0
	} else if sec >= 260 {
		r.crashRobot()
	}
	r.clampAll()
}

func (r *Robot) crashRobot() {
	r.Status = "OFFLINE"
	r.CrashPublished = false
	for key := range r.Sensors {
		r.Sensors[key] = 0
	}
}

func (r *Robot) recoverRobot() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Status = "ONLINE"
	r.BehaviorType = "stable"
	r.StartTime = time.Now()
	r.CrashPublished = false
	for key, value := range r.Initial {
		r.Sensors[key] = value
	}
}

func cloneSensors(input map[string]float64) map[string]float64 {
	output := make(map[string]float64, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func startControlServer(robots []*Robot) {
	byID := map[string]*Robot{}
	for _, robot := range robots {
		byID[robot.ID] = robot
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/robots", func(w http.ResponseWriter, r *http.Request) {
		writeCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		type robotView struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Role   string `json:"role"`
			Status string `json:"status"`
		}
		result := []robotView{}
		for _, robot := range robots {
			result = append(result, robotView{ID: robot.ID, Name: robot.Name, Role: robot.Role, Status: robot.Status})
		}
		json.NewEncoder(w).Encode(result)
	})
	mux.HandleFunc("/crash", func(w http.ResponseWriter, r *http.Request) {
		writeCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		robot := byID[r.URL.Query().Get("nodeId")]
		if robot == nil {
			http.Error(w, "robot not found", http.StatusNotFound)
			return
		}
		robot.mu.Lock()
		robot.crashRobot()
		robot.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"status": "crashed", "nodeId": robot.ID})
	})
	mux.HandleFunc("/recover", func(w http.ResponseWriter, r *http.Request) {
		writeCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		robot := byID[r.URL.Query().Get("nodeId")]
		if robot == nil {
			http.Error(w, "robot not found", http.StatusNotFound)
			return
		}
		robot.recoverRobot()
		json.NewEncoder(w).Encode(map[string]string{"status": "online", "nodeId": robot.ID})
	})

	fmt.Println("   Control: http://localhost:9100")
	if err := http.ListenAndServe(":9100", mux); err != nil {
		fmt.Printf("control server error: %v\n", err)
	}
}

func writeCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
}

func (r *Robot) clampSensor(name string) {
	cfg := sensorConfigs[name]
	r.Sensors[name] = math.Max(cfg.Min, math.Min(cfg.Max, r.Sensors[name]))
}

func (r *Robot) clampAll() {
	for sensorName := range r.Sensors {
		r.clampSensor(sensorName)
	}
}

func randNormal() float64 {
	return rand.NormFloat64()
}
