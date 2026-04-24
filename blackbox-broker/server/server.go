package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	aiClient "github.com/blackbox/broker/ai"
	"github.com/blackbox/broker/anomaly"
	"github.com/blackbox/broker/models"
	"github.com/blackbox/broker/pubsub"
	"github.com/blackbox/broker/storage"
	"github.com/blackbox/broker/watchdog"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // allow all origins for hackathon
}

// Server holds all dependencies and registers routes.
type Server struct {
	db        *storage.DB
	bus       *pubsub.Bus
	watchdog  *watchdog.Watchdog
	anomalyEng *anomaly.Engine
	ai        *aiClient.Client
	startTime time.Time
	msgCount  *atomic.Int64 // shared with broker pipeline
}

// New wires up all dependencies.
func New(
	db *storage.DB,
	bus *pubsub.Bus,
	wd *watchdog.Watchdog,
	ae *anomaly.Engine,
	ai *aiClient.Client,
	msgCount *atomic.Int64,
) *Server {
	return &Server{
		db:         db,
		bus:        bus,
		watchdog:   wd,
		anomalyEng: ae,
		ai:         ai,
		startTime:  time.Now(),
		msgCount:   msgCount,
	}
}

// Start registers all routes and begins listening.
func (s *Server) Start(port int) error {
	mux := http.NewServeMux()

	// WebSocket
	mux.HandleFunc("/ws", s.handleWS)

	// System
	mux.HandleFunc("/stats", s.handleStats)

	// Nodes
	mux.HandleFunc("/nodes", s.handleNodes)

	// Replay  GET /replay?nodeId=arduino-1&topic=/distance&from=1700000000000&to=1700003600000&limit=5000
	mux.HandleFunc("/replay", s.handleReplay)

	// Sessions
	mux.HandleFunc("/sessions", s.handleSessions)

	// Anomalies  GET /anomalies?nodeId=arduino-1&limit=50
	mux.HandleFunc("/anomalies", s.handleAnomalies)

	// Crash events  GET /crashes?nodeId=arduino-1&resolved=false&limit=20
	mux.HandleFunc("/crashes", s.handleCrashes)
	// POST /crashes/{id}/resolve
	mux.HandleFunc("/crashes/resolve", s.handleResolveCrash)
	// POST /crashes/{id}/analyze  — trigger AI analysis manually
	mux.HandleFunc("/crashes/analyze", s.handleAnalyzeCrash)

	// Topics  GET /topics
	mux.HandleFunc("/topics", s.handleTopics)

	// Export  GET /export?nodeId=arduino-1
	mux.HandleFunc("/export", s.handleExport)

	// Threshold update  POST /thresholds  body: {"topic":"/tilt","min":-30,"max":30}
	mux.HandleFunc("/thresholds", s.handleThresholds)

	// Periodic system stats broadcast
	go s.broadcastStats()

	addr := fmt.Sprintf(":%d", port)
	log.Printf("[server] listening on %s", addr)
	return http.ListenAndServe(addr, cors(mux))
}

// ── WebSocket ────────────────────────────────────────────────────────────────

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade: %v", err)
		return
	}
	subID := fmt.Sprintf("ws-%d", time.Now().UnixNano())
	sub := s.bus.Subscribe(subID)
	defer func() {
		s.bus.Unsubscribe(subID)
		conn.Close()
	}()
	log.Printf("[ws] client connected: %s", subID)

	// Write loop
	go func() {
		for ev := range sub.Chan {
			data, _ := json.Marshal(ev)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}()

	// Read loop (keep-alive / ping-pong)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		default:
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}
}

// ── REST helpers ─────────────────────────────────────────────────────────────

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := models.SystemStats{
		TotalMessages:  s.msgCount.Load(),
		MessagesPerSec: 0, // computed in broadcastStats
		DBSizeBytes:    s.db.FileSize(),
		UptimeSeconds:  int64(time.Since(s.startTime).Seconds()),
		ActiveNodes:    s.watchdog.ActiveCount(),
		NodeStatuses:   s.watchdog.Statuses(),
	}
	writeJSON(w, stats)
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.db.AllNodes()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, nodes)
}

func (s *Server) handleReplay(w http.ResponseWriter, r *http.Request) {
	nodeIDStr := r.URL.Query().Get("nodeId")
	topic := r.URL.Query().Get("topic")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	limitStr := r.URL.Query().Get("limit")

	from, _ := strconv.ParseInt(fromStr, 10, 64)
	to, _ := strconv.ParseInt(toStr, 10, 64)
	limit, _ := strconv.Atoi(limitStr)
	if to == 0 {
		to = time.Now().UnixMilli()
	}
	if limit == 0 {
		limit = 10000
	}

	nodeDBID, err := s.db.GetNodeID(nodeIDStr)
	if err != nil {
		http.Error(w, "node not found: "+nodeIDStr, 404)
		return
	}

	rows, err := s.db.ReplayQuery(nodeDBID, topic, from, to, limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, rows)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.db.GetSessions()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, sessions)
}

func (s *Server) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	nodeIDStr := r.URL.Query().Get("nodeId")
	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	if limit == 0 {
		limit = 50
	}
	nodeDBID, err := s.db.GetNodeID(nodeIDStr)
	if err != nil {
		http.Error(w, "node not found", 404)
		return
	}
	anomalies, err := s.db.RecentAnomalies(nodeDBID, limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, anomalies)
}

func (s *Server) handleCrashes(w http.ResponseWriter, r *http.Request) {
	nodeIDStr := r.URL.Query().Get("nodeId")
	resolvedStr := r.URL.Query().Get("resolved")
	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	if limit == 0 {
		limit = 20
	}

	nodeDBID, err := s.db.GetNodeID(nodeIDStr)
	if err != nil {
		http.Error(w, "node not found", 404)
		return
	}

	var resolved *bool
	if resolvedStr != "" {
		b, _ := strconv.ParseBool(resolvedStr)
		resolved = &b
	}

	crashes, err := s.db.GetCrashEvents(nodeDBID, resolved, limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, crashes)
}

func (s *Server) handleResolveCrash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}
	if err := s.db.ResolveCrashEvent(id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handleAnalyzeCrash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	// Decode body: {crashId, nodeId, windowSec}
	var body struct {
		CrashID   int64  `json:"crashId"`
		NodeID    string `json:"nodeId"`
		WindowSec int    `json:"windowSec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad body", 400)
		return
	}
	if body.WindowSec == 0 {
		body.WindowSec = 30
	}

	nodeDBID, err := s.db.GetNodeID(body.NodeID)
	if err != nil {
		http.Error(w, "node not found", 404)
		return
	}

	// Fetch the crash event
	crashes, err := s.db.GetCrashEvents(nodeDBID, nil, 1000)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var crash *models.CrashEvent
	for i, c := range crashes {
		if c.ID == body.CrashID {
			crash = &crashes[i]
			break
		}
	}
	if crash == nil {
		http.Error(w, "crash event not found", 404)
		return
	}

	// Get the surrounding sensor data
	from := crash.Timestamp - int64(body.WindowSec)*1000
	rows, _ := s.db.ReplayQuery(nodeDBID, "", from, crash.Timestamp, 500)

	// Call Groq
	result, err := s.ai.AnalyzeCrash(*crash, rows)
	if err != nil {
		http.Error(w, "AI error: "+err.Error(), 500)
		return
	}

	// Persist the AI result back to the crash event
	s.db.UpdateCrashEventAI(crash.ID, result.RootCause, result.Suggestion, result.Confidence)

	// Broadcast updated crash event over WebSocket
	crash.AIRootCause = result.RootCause
	crash.AISuggestion = result.Suggestion
	crash.Confidence = result.Confidence
	s.bus.Publish(models.WSEvent{Type: models.EventCrash, Payload: crash})

	writeJSON(w, result)
}

func (s *Server) handleTopics(w http.ResponseWriter, r *http.Request) {
	topics, err := s.db.AllTopics()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, topics)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	nodeIDStr := r.URL.Query().Get("nodeId")
	nodeDBID, err := s.db.GetNodeID(nodeIDStr)
	if err != nil {
		http.Error(w, "node not found", 404)
		return
	}
	csv, err := s.db.ExportCSV(nodeDBID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="blackbox-%s.csv"`, nodeIDStr))
	w.Write([]byte(csv))
}

func (s *Server) handleThresholds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var body struct {
		Topic string  `json:"topic"`
		Min   float64 `json:"min"`
		Max   float64 `json:"max"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad body", 400)
		return
	}
	s.anomalyEng.UpdateThreshold(body.Topic, body.Min, body.Max)
	writeJSON(w, map[string]string{"status": "updated"})
}

// ── Periodic stats broadcast ─────────────────────────────────────────────────

func (s *Server) broadcastStats() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var lastCount int64
	var lastTime = time.Now()

	for range ticker.C {
		now := time.Now()
		current := s.msgCount.Load()
		elapsed := now.Sub(lastTime).Seconds()
		rate := float64(current-lastCount) / elapsed
		lastCount = current
		lastTime = now

		stats := models.SystemStats{
			TotalMessages:  current,
			MessagesPerSec: rate,
			DBSizeBytes:    s.db.FileSize(),
			UptimeSeconds:  int64(time.Since(s.startTime).Seconds()),
			ActiveNodes:    s.watchdog.ActiveCount(),
			NodeStatuses:   s.watchdog.Statuses(),
		}
		s.bus.Publish(models.WSEvent{Type: models.EventSystemStats, Payload: stats})
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// cors adds permissive CORS headers for local dev / hackathon use.
func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		h.ServeHTTP(w, r)
	})
}
