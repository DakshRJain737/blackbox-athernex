package storage

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/blackbox/broker/models"
	_ "github.com/mattn/go-sqlite3"
)

// DB wraps *sql.DB and exposes typed methods for every entity.
type DB struct {
	conn *sql.DB
	path string
}

// Open opens (or creates) the SQLite database and runs migrations.
func Open(path string) (*DB, error) {
	// WAL mode: allow concurrent reads while writing
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	conn.SetMaxOpenConns(1) // SQLite is single-writer

	db := &DB{conn: conn, path: path}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// Close closes the underlying connection.
func (db *DB) Close() { db.conn.Close() }

// ── Migration ────────────────────────────────────────────────────────────────

func (db *DB) migrate() error {
	ddl := `
	-- mirrors sensor_topics entity
	CREATE TABLE IF NOT EXISTS sensor_topics (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		topic_name    TEXT    NOT NULL UNIQUE,
		unit          TEXT    NOT NULL DEFAULT '',
		color         TEXT    NOT NULL DEFAULT '#ffffff',
		threshold_min REAL    NOT NULL DEFAULT 0,
		threshold_max REAL    NOT NULL DEFAULT 9999,
		created_at    INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000),
		updated_at    INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000)
	);

	-- mirrors nodes entity
	CREATE TABLE IF NOT EXISTS nodes (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id    TEXT    NOT NULL UNIQUE,
		name       TEXT    NOT NULL DEFAULT '',
		user_id    INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000),
		updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000)
	);

	-- mirrors sensor_data entity
	CREATE TABLE IF NOT EXISTS sensor_data (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		value      REAL    NOT NULL,
		timestamp  INTEGER NOT NULL,
		topic_id   INTEGER NOT NULL REFERENCES sensor_topics(id),
		node_id    INTEGER NOT NULL REFERENCES nodes(id),
		created_at INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000),
		updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000)
	);
	CREATE INDEX IF NOT EXISTS idx_sensor_node_time ON sensor_data(node_id, timestamp);

	-- mirrors anomalies entity
	CREATE TABLE IF NOT EXISTS anomalies (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		topic      TEXT    NOT NULL,
		value      REAL    NOT NULL,
		threshold  REAL    NOT NULL,
		timestamp  INTEGER NOT NULL,
		node_id    INTEGER NOT NULL REFERENCES nodes(id),
		created_at INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000),
		updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000)
	);
	CREATE INDEX IF NOT EXISTS idx_anomaly_node_time ON anomalies(node_id, timestamp);

	-- mirrors crash_events entity
	CREATE TABLE IF NOT EXISTS crash_events (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		topic         TEXT    NOT NULL,
		value         REAL    NOT NULL,
		threshold     REAL    NOT NULL,
		timestamp     INTEGER NOT NULL,
		severity      TEXT    NOT NULL DEFAULT 'LOW',
		ai_root_cause TEXT    NOT NULL DEFAULT '',
		ai_suggestion TEXT    NOT NULL DEFAULT '',
		confidence    REAL    NOT NULL DEFAULT 0,
		resolved      INTEGER NOT NULL DEFAULT 0,
		node_id       INTEGER NOT NULL REFERENCES nodes(id),
		created_at    INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000),
		updated_at    INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000)
	);
	CREATE INDEX IF NOT EXISTS idx_crash_node_time       ON crash_events(node_id, timestamp);
	CREATE INDEX IF NOT EXISTS idx_crash_node_threshold  ON crash_events(node_id, threshold);
	CREATE INDEX IF NOT EXISTS idx_crash_node_confidence ON crash_events(node_id, confidence);

	-- mirrors replay_sessions entity
	CREATE TABLE IF NOT EXISTS replay_sessions (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		start_time     INTEGER NOT NULL,
		end_time       INTEGER NOT NULL DEFAULT 0,
		session_name   TEXT    NOT NULL DEFAULT '',
		total_messages INTEGER NOT NULL DEFAULT 0,
		file_path      TEXT    NOT NULL DEFAULT '',
		is_active      INTEGER NOT NULL DEFAULT 0,
		user_id        INTEGER NOT NULL DEFAULT 0,
		created_at     INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000),
		updated_at     INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000)
	);

	-- schema_errors table for malformed messages
	CREATE TABLE IF NOT EXISTS schema_errors (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		raw_payload  TEXT    NOT NULL,
		error_reason TEXT    NOT NULL,
		timestamp    INTEGER NOT NULL,
		node_id      TEXT    NOT NULL DEFAULT '',
		created_at   INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000)
	);
	`
	_, err := db.conn.Exec(ddl)
	return err
}

// ── SensorTopic ──────────────────────────────────────────────────────────────

// UpsertTopic inserts or updates a SensorTopic row and returns the row id.
func (db *DB) UpsertTopic(t models.SensorTopic) (int64, error) {
	now := time.Now().UnixMilli()
	res, err := db.conn.Exec(`
		INSERT INTO sensor_topics (topic_name, unit, color, threshold_min, threshold_max, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(topic_name) DO UPDATE SET
			unit=excluded.unit, color=excluded.color,
			threshold_min=excluded.threshold_min, threshold_max=excluded.threshold_max,
			updated_at=excluded.updated_at`,
		t.TopicName, t.Unit, t.Color, t.ThresholdMin, t.ThresholdMax, now, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		row := db.conn.QueryRow(`SELECT id FROM sensor_topics WHERE topic_name=?`, t.TopicName)
		row.Scan(&id)
	}
	return id, nil
}

// GetTopicByName returns the db row id for a topic name.
func (db *DB) GetTopicID(name string) (int64, error) {
	var id int64
	err := db.conn.QueryRow(`SELECT id FROM sensor_topics WHERE topic_name=?`, name).Scan(&id)
	return id, err
}

func (db *DB) AllTopics() ([]models.SensorTopic, error) {
	rows, err := db.conn.Query(`SELECT id, topic_name, unit, color, threshold_min, threshold_max FROM sensor_topics`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.SensorTopic
	for rows.Next() {
		var t models.SensorTopic
		rows.Scan(&t.ID, &t.TopicName, &t.Unit, &t.Color, &t.ThresholdMin, &t.ThresholdMax)
		out = append(out, t)
	}
	return out, nil
}

// ── Node ─────────────────────────────────────────────────────────────────────

// UpsertNode inserts or updates a node row and returns its db id.
func (db *DB) UpsertNode(nodeID, name string) (int64, error) {
	now := time.Now().UnixMilli()
	res, err := db.conn.Exec(`
		INSERT INTO nodes (node_id, name, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(node_id) DO UPDATE SET name=excluded.name, updated_at=excluded.updated_at`,
		nodeID, name, now, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		row := db.conn.QueryRow(`SELECT id FROM nodes WHERE node_id=?`, nodeID)
		row.Scan(&id)
	}
	return id, nil
}

func (db *DB) GetNodeID(nodeID string) (int64, error) {
	var id int64
	err := db.conn.QueryRow(`SELECT id FROM nodes WHERE node_id=?`, nodeID).Scan(&id)
	return id, err
}

func (db *DB) AllNodes() ([]models.Node, error) {
	rows, err := db.conn.Query(`SELECT id, node_id, name, user_id, created_at, updated_at FROM nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Node
	for rows.Next() {
		var n models.Node
		rows.Scan(&n.ID, &n.NodeID, &n.Name, &n.UserID, &n.CreatedAt, &n.UpdatedAt)
		out = append(out, n)
	}
	return out, nil
}

// ── SensorData ───────────────────────────────────────────────────────────────

// SaveSensorData persists one sensor reading.
func (db *DB) SaveSensorData(nodeDBID, topicDBID int64, value float64, ts int64) (int64, error) {
	now := time.Now().UnixMilli()
	res, err := db.conn.Exec(`
		INSERT INTO sensor_data (value, timestamp, topic_id, node_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		value, ts, topicDBID, nodeDBID, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ReplayQuery returns sensor_data rows for a node+topic window (inclusive).
// Pass topicName="" to get all topics.
func (db *DB) ReplayQuery(nodeDBID int64, topicName string, from, to int64, limit int) ([]map[string]interface{}, error) {
	var rows *sql.Rows
	var err error

	if limit <= 0 {
		limit = 10000
	}

	if topicName == "" {
		rows, err = db.conn.Query(`
			SELECT sd.id, sd.value, sd.timestamp, st.topic_name, st.unit, st.color
			FROM sensor_data sd
			JOIN sensor_topics st ON sd.topic_id = st.id
			WHERE sd.node_id=? AND sd.timestamp BETWEEN ? AND ?
			ORDER BY sd.timestamp ASC LIMIT ?`,
			nodeDBID, from, to, limit)
	} else {
		rows, err = db.conn.Query(`
			SELECT sd.id, sd.value, sd.timestamp, st.topic_name, st.unit, st.color
			FROM sensor_data sd
			JOIN sensor_topics st ON sd.topic_id = st.id
			WHERE sd.node_id=? AND st.topic_name=? AND sd.timestamp BETWEEN ? AND ?
			ORDER BY sd.timestamp ASC LIMIT ?`,
			nodeDBID, topicName, from, to, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]interface{}
	for rows.Next() {
		var id int64
		var value float64
		var ts int64
		var topicN, unit, color string
		rows.Scan(&id, &value, &ts, &topicN, &unit, &color)
		out = append(out, map[string]interface{}{
			"id": id, "value": value, "timestamp": ts,
			"topic": topicN, "unit": unit, "color": color,
		})
	}
	return out, nil
}

// TotalMessages returns total sensor_data row count.
func (db *DB) TotalMessages() int64 {
	var count int64
	db.conn.QueryRow(`SELECT COUNT(*) FROM sensor_data`).Scan(&count)
	return count
}

// ── Anomaly ──────────────────────────────────────────────────────────────────

func (db *DB) SaveAnomaly(a models.Anomaly) (int64, error) {
	now := time.Now().UnixMilli()
	res, err := db.conn.Exec(`
		INSERT INTO anomalies (topic, value, threshold, timestamp, node_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.Topic, a.Value, a.Threshold, a.Timestamp, a.NodeID, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) RecentAnomalies(nodeDBID int64, limit int) ([]models.Anomaly, error) {
	rows, err := db.conn.Query(`
		SELECT id, topic, value, threshold, timestamp, node_id
		FROM anomalies WHERE node_id=?
		ORDER BY timestamp DESC LIMIT ?`, nodeDBID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Anomaly
	for rows.Next() {
		var a models.Anomaly
		rows.Scan(&a.ID, &a.Topic, &a.Value, &a.Threshold, &a.Timestamp, &a.NodeID)
		out = append(out, a)
	}
	return out, nil
}

// ── CrashEvent ───────────────────────────────────────────────────────────────

func (db *DB) SaveCrashEvent(c models.CrashEvent) (int64, error) {
	now := time.Now().UnixMilli()
	res, err := db.conn.Exec(`
		INSERT INTO crash_events
			(topic, value, threshold, timestamp, severity, ai_root_cause, ai_suggestion, confidence, resolved, node_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Topic, c.Value, c.Threshold, c.Timestamp,
		string(c.Severity), c.AIRootCause, c.AISuggestion, c.Confidence,
		boolToInt(c.Resolved), c.NodeID, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) UpdateCrashEventAI(id int64, rootCause, suggestion string, confidence float64) error {
	now := time.Now().UnixMilli()
	_, err := db.conn.Exec(`
		UPDATE crash_events SET ai_root_cause=?, ai_suggestion=?, confidence=?, updated_at=?
		WHERE id=?`, rootCause, suggestion, confidence, now, id)
	return err
}

func (db *DB) ResolveCrashEvent(id int64) error {
	now := time.Now().UnixMilli()
	_, err := db.conn.Exec(`UPDATE crash_events SET resolved=1, updated_at=? WHERE id=?`, now, id)
	return err
}

func (db *DB) GetCrashEvents(nodeDBID int64, resolved *bool, limit int) ([]models.CrashEvent, error) {
	query := `SELECT id, topic, value, threshold, timestamp, severity, ai_root_cause, ai_suggestion, confidence, resolved, node_id
		FROM crash_events WHERE node_id=?`
	args := []interface{}{nodeDBID}
	if resolved != nil {
		query += ` AND resolved=?`
		args = append(args, boolToInt(*resolved))
	}
	query += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.CrashEvent
	for rows.Next() {
		var c models.CrashEvent
		var sev string
		var res int
		rows.Scan(&c.ID, &c.Topic, &c.Value, &c.Threshold, &c.Timestamp,
			&sev, &c.AIRootCause, &c.AISuggestion, &c.Confidence, &res, &c.NodeID)
		c.Severity = models.Severity(sev)
		c.Resolved = res == 1
		out = append(out, c)
	}
	return out, nil
}

// ── ReplaySession ─────────────────────────────────────────────────────────────

func (db *DB) StartSession(name string) (int64, error) {
	now := time.Now().UnixMilli()
	res, err := db.conn.Exec(`
		INSERT INTO replay_sessions (start_time, session_name, is_active, created_at, updated_at)
		VALUES (?, ?, 1, ?, ?)`, now, name, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) EndSession(id int64, totalMessages int) error {
	now := time.Now().UnixMilli()
	_, err := db.conn.Exec(`
		UPDATE replay_sessions SET end_time=?, total_messages=?, is_active=0, updated_at=?
		WHERE id=?`, now, totalMessages, now, id)
	return err
}

func (db *DB) GetSessions() ([]models.ReplaySession, error) {
	rows, err := db.conn.Query(`
		SELECT id, start_time, end_time, session_name, total_messages, file_path, is_active, user_id
		FROM replay_sessions ORDER BY start_time DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ReplaySession
	for rows.Next() {
		var s models.ReplaySession
		var active int
		rows.Scan(&s.ID, &s.StartTime, &s.EndTime, &s.SessionName,
			&s.TotalMessages, &s.FilePath, &active, &s.UserID)
		s.IsActive = active == 1
		out = append(out, s)
	}
	return out, nil
}

// ── SchemaError ───────────────────────────────────────────────────────────────

func (db *DB) SaveSchemaError(e models.SchemaError) error {
	_, err := db.conn.Exec(`
		INSERT INTO schema_errors (raw_payload, error_reason, timestamp, node_id, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		e.RawPayload, e.ErrorReason, e.Timestamp, e.NodeID, time.Now().UnixMilli())
	return err
}

// ── File size helper ──────────────────────────────────────────────────────────

func (db *DB) FileSize() int64 {
	fi, err := os.Stat(db.path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

// ── Export ────────────────────────────────────────────────────────────────────

// ExportCSV writes all sensor_data for a node to a CSV string.
func (db *DB) ExportCSV(nodeDBID int64) (string, error) {
	rows, err := db.conn.Query(`
		SELECT sd.timestamp, st.topic_name, sd.value, st.unit
		FROM sensor_data sd
		JOIN sensor_topics st ON sd.topic_id = st.id
		WHERE sd.node_id=?
		ORDER BY sd.timestamp ASC`, nodeDBID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	csv := "timestamp,topic,value,unit\n"
	for rows.Next() {
		var ts int64
		var topic, unit string
		var value float64
		rows.Scan(&ts, &topic, &value, &unit)
		csv += fmt.Sprintf("%d,%s,%f,%s\n", ts, topic, value, unit)
	}
	return csv, nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SeedTopics inserts the default SensorTopic rows from config.
// Called once on startup so the ids exist before any messages arrive.
func (db *DB) SeedTopics(topics []models.SensorTopic) error {
	for _, t := range topics {
		if _, err := db.UpsertTopic(t); err != nil {
			log.Printf("seed topic %s: %v", t.TopicName, err)
		}
	}
	return nil
}
