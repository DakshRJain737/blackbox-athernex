package models

// ── Severity mirrors the Java enum ──────────────────────────────────────────

type Severity string

const (
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

func (s Severity) Message() string {
	switch s {
	case SeverityLow:
		return "The crash event is of low severity"
	case SeverityMedium:
		return "The crash event is of medium severity"
	case SeverityHigh:
		return "The crash event is of high severity"
	case SeverityCritical:
		return "The crash event is of critical severity"
	default:
		return "Unknown severity"
	}
}

// ── SensorTopic matches sensor_topics table ──────────────────────────────────

type SensorTopic struct {
	ID           int64   `json:"id"`
	TopicName    string  `json:"topicName"`
	Unit         string  `json:"unit"`
	Color        string  `json:"color"`
	ThresholdMin float64 `json:"thresholdMin"`
	ThresholdMax float64 `json:"thresholdMax"`
}

// ── Node matches nodes table ─────────────────────────────────────────────────

type Node struct {
	ID        int64  `json:"id"`
	NodeID    string `json:"nodeId"`
	Name      string `json:"name"`
	UserID    int64  `json:"userId"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// NodeStatus is the live runtime status (not persisted, sent over WebSocket)
type NodeStatus struct {
	NodeID       string  `json:"nodeId"`
	IsAlive      bool    `json:"isAlive"`
	LastSeen     int64   `json:"lastSeen"`
	MessageRate  int     `json:"messageRate"`
	TotalMessages int64  `json:"totalMessages"`
	Status       string  `json:"status"`
	CPUUsage     float64 `json:"cpuUsage"`
	MemoryUsage  float64 `json:"memoryUsage"`
}

// ── SensorData matches sensor_data table ────────────────────────────────────

type SensorData struct {
	ID        int64       `json:"id"`
	Value     float64     `json:"value"`
	Timestamp int64       `json:"timestamp"` // Unix ms — set by broker on receipt
	Topic     SensorTopic `json:"topic"`
	NodeID    int64       `json:"nodeId"`
}

// RawMessage is what arrives over serial from an Arduino
type RawMessage struct {
	NodeID    string  `json:"nodeId"`
	Topic     string  `json:"topic"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit,omitempty"`
}

// ── Anomaly matches anomalies table ─────────────────────────────────────────

type Anomaly struct {
	ID        int64   `json:"id"`
	Topic     string  `json:"topic"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Timestamp int64   `json:"timestamp"`
	NodeID    int64   `json:"nodeId"`
}

// ── CrashEvent matches crash_events table ────────────────────────────────────

type CrashEvent struct {
	ID          int64    `json:"id"`
	Topic       string   `json:"topic"`
	Value       float64  `json:"value"`
	Threshold   float64  `json:"threshold"`
	Timestamp   int64    `json:"timestamp"`
	Severity    Severity `json:"severity"`
	AIRootCause string   `json:"aiRootCause"`
	AISuggestion string  `json:"aiSuggestion"`
	Confidence  float64  `json:"confidence"`
	Resolved    bool     `json:"resolved"`
	NodeID      int64    `json:"nodeId"`
}

// ── ReplaySession matches replay_sessions table ───────────────────────────────

type ReplaySession struct {
	ID            int64  `json:"id"`
	StartTime     int64  `json:"startTime"`
	EndTime       int64  `json:"endTime"`
	SessionName   string `json:"sessionName"`
	TotalMessages int    `json:"totalMessages"`
	FilePath      string `json:"filePath"`
	IsActive      bool   `json:"isActive"`
	UserID        int64  `json:"userId"`
}

// ── SchemaError is logged for every malformed message ────────────────────────

type SchemaError struct {
	ID        int64  `json:"id"`
	RawPayload string `json:"rawPayload"`
	ErrorReason string `json:"errorReason"`
	Timestamp  int64  `json:"timestamp"`
	NodeID     string `json:"nodeId"`
}

// ── WebSocket event envelope ─────────────────────────────────────────────────

type WSEventType string

const (
	EventMessage    WSEventType = "MESSAGE"
	EventAnomaly    WSEventType = "ANOMALY"
	EventCrash      WSEventType = "CRASH"
	EventNodeDeath  WSEventType = "NODE_DEATH"
	EventNodeAlive  WSEventType = "NODE_ALIVE"
	EventSchemaError WSEventType = "SCHEMA_ERROR"
	EventSystemStats WSEventType = "SYSTEM_STATS"
)

type WSEvent struct {
	Type    WSEventType `json:"type"`
	Payload interface{} `json:"payload"`
}

// ── SystemStats is broadcast periodically ────────────────────────────────────

type SystemStats struct {
	TotalMessages   int64              `json:"totalMessages"`
	MessagesPerSec  float64            `json:"messagesPerSec"`
	DBSizeBytes     int64              `json:"dbSizeBytes"`
	UptimeSeconds   int64              `json:"uptimeSeconds"`
	ActiveNodes     int                `json:"activeNodes"`
	NodeStatuses    []NodeStatus       `json:"nodeStatuses"`
}
