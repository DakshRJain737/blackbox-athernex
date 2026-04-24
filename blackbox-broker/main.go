package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	aiClient "github.com/blackbox/broker/ai"
	"github.com/blackbox/broker/anomaly"
	"github.com/blackbox/broker/config"
	"github.com/blackbox/broker/models"
	"github.com/blackbox/broker/pubsub"
	"github.com/blackbox/broker/serial"
	"github.com/blackbox/broker/server"
	"github.com/blackbox/broker/storage"
	"github.com/blackbox/broker/watchdog"
)

func main() {
	cfgPath := flag.String("config", "config/config.json", "path to config.json")
	flag.Parse()

	// ── Load config ──────────────────────────────────────────────────────────
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	log.Printf("[main] config loaded — %d serial ports, %d topics", len(cfg.SerialPorts), len(cfg.Topics))

	// ── Open database ────────────────────────────────────────────────────────
	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Seed SensorTopic rows from config so IDs exist before any messages arrive
	topicModels := make([]models.SensorTopic, len(cfg.Topics))
	for i, t := range cfg.Topics {
		topicModels[i] = models.SensorTopic{
			TopicName:    t.TopicName,
			Unit:         t.Unit,
			Color:        t.Color,
			ThresholdMin: t.ThresholdMin,
			ThresholdMax: t.ThresholdMax,
		}
	}
	if err := db.SeedTopics(topicModels); err != nil {
		log.Fatalf("seed topics: %v", err)
	}

	// Register nodes from config
	for _, p := range cfg.SerialPorts {
		if _, err := db.UpsertNode(p.NodeID, p.Name); err != nil {
			log.Printf("[main] upsert node %s: %v", p.NodeID, err)
		}
	}

	// ── Start a new ReplaySession ────────────────────────────────────────────
	sessionID, err := db.StartSession("session-" + time.Now().Format("2006-01-02T15-04-05"))
	if err != nil {
		log.Printf("[main] start session: %v", err)
	}

	// ── Pub/Sub bus ──────────────────────────────────────────────────────────
	bus := pubsub.New()

	// ── Watchdog ─────────────────────────────────────────────────────────────
	wd := watchdog.New(cfg.NodeDeadTimeout, bus)
	wd.Start()

	// ── Anomaly engine ───────────────────────────────────────────────────────
	ae := anomaly.New(topicModels, cfg.AnomalyCooldown)

	// ── AI client ────────────────────────────────────────────────────────────
	ai := aiClient.New(cfg.GroqAPIKey, cfg.GroqModel)

	// ── Message counter (shared with server for stats) ───────────────────────
	var msgCount atomic.Int64

	// ── Topic-ID cache to avoid DB lookup per message ────────────────────────
	topicIDCache := make(map[string]int64)
	for _, t := range topicModels {
		id, err := db.GetTopicID(t.TopicName)
		if err == nil {
			topicIDCache[t.TopicName] = id
		}
	}

	// ── Node-DB-ID cache ─────────────────────────────────────────────────────
	nodeDBIDCache := make(map[string]int64)
	for _, p := range cfg.SerialPorts {
		id, err := db.GetNodeID(p.NodeID)
		if err == nil {
			nodeDBIDCache[p.NodeID] = id
		}
	}

	// ── Message handler ───────────────────────────────────────────────────────
	// Called by every serial reader goroutine (concurrent-safe because DB uses
	// a single connection and all maps are pre-populated before serial starts).
	onMessage := func(msg models.RawMessage, ts int64) {
		msgCount.Add(1)

		// 1. Heartbeat the watchdog
		wd.Beat(msg.NodeID, msg.NodeID)

		// 2. Resolve DB IDs (dynamic topics get upserted on first sight)
		topicID, ok := topicIDCache[msg.Topic]
		if !ok {
			id, err := db.UpsertTopic(models.SensorTopic{TopicName: msg.Topic})
			if err != nil {
				log.Printf("[broker] upsert topic %s: %v", msg.Topic, err)
				return
			}
			topicIDCache[msg.Topic] = id
			topicID = id
		}

		nodeDBID, ok := nodeDBIDCache[msg.NodeID]
		if !ok {
			id, err := db.UpsertNode(msg.NodeID, msg.NodeID)
			if err != nil {
				log.Printf("[broker] upsert node %s: %v", msg.NodeID, err)
				return
			}
			nodeDBIDCache[msg.NodeID] = id
			nodeDBID = id
		}

		// 3. Persist sensor data
		if _, err := db.SaveSensorData(nodeDBID, topicID, msg.Value, ts); err != nil {
			log.Printf("[broker] save sensor: %v", err)
		}

		// 4. Broadcast live message over WebSocket
		bus.Publish(models.WSEvent{Type: models.EventMessage, Payload: msg})

		// 5. Anomaly check
		result := ae.Check(msg.Topic, msg.Value)
		if result.IsAnomaly {
			anom := models.Anomaly{
				Topic:     msg.Topic,
				Value:     msg.Value,
				Threshold: result.Threshold,
				Timestamp: ts,
				NodeID:    nodeDBID,
			}
			anomID, err := db.SaveAnomaly(anom)
			if err != nil {
				log.Printf("[broker] save anomaly: %v", err)
			}
			anom.ID = anomID

			// Broadcast anomaly event
			bus.Publish(models.WSEvent{Type: models.EventAnomaly, Payload: anom})

			// Persist crash event
			crash := models.CrashEvent{
				Topic:     msg.Topic,
				Value:     msg.Value,
				Threshold: result.Threshold,
				Timestamp: ts,
				Severity:  result.Severity,
				NodeID:    nodeDBID,
			}
			crashID, err := db.SaveCrashEvent(crash)
			if err != nil {
				log.Printf("[broker] save crash: %v", err)
			}
			crash.ID = crashID

			// Trigger AI analysis asynchronously — don't block the message pipeline
			go func(c models.CrashEvent, nid int64) {
				from := ts - 30000 // 30 seconds before crash
				rows, _ := db.ReplayQuery(nid, "", from, ts, 500)
				aiResult, err := ai.AnalyzeCrash(c, rows)
				if err != nil {
					log.Printf("[ai] analyze: %v", err)
					return
				}
				db.UpdateCrashEventAI(c.ID, aiResult.RootCause, aiResult.Suggestion, aiResult.Confidence)
				c.AIRootCause = aiResult.RootCause
				c.AISuggestion = aiResult.Suggestion
				c.Confidence = aiResult.Confidence
				bus.Publish(models.WSEvent{Type: models.EventCrash, Payload: c})
				log.Printf("[ai] crash %d analysed: %s", c.ID, aiResult.RootCause)
			}(crash, nodeDBID)
		}
	}

	// ── Schema error handler ──────────────────────────────────────────────────
	onError := func(e models.SchemaError) {
		log.Printf("[schema] error from %s: %s | raw: %q", e.NodeID, e.ErrorReason, e.RawPayload)
		if err := db.SaveSchemaError(e); err != nil {
			log.Printf("[schema] save error: %v", err)
		}
		bus.Publish(models.WSEvent{Type: models.EventSchemaError, Payload: e})
	}

	// ── Start serial readers ──────────────────────────────────────────────────
	serial.StartAll(cfg.SerialPorts, onMessage, onError)
	log.Printf("[main] serial readers started for %d ports", len(cfg.SerialPorts))

	// ── Start HTTP / WebSocket server ─────────────────────────────────────────
	srv := server.New(db, bus, wd, ae, ai, &msgCount)
	go func() {
		if err := srv.Start(cfg.HTTPPort); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	// ── Wait for SIGINT / SIGTERM ─────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[main] shutting down...")
	total := int(msgCount.Load())
	if err := db.EndSession(sessionID, total); err != nil {
		log.Printf("[main] end session: %v", err)
	}
	log.Printf("[main] session closed — %d messages logged", total)
}
