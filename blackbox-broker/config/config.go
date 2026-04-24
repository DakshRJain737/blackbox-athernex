package config

import (
	"encoding/json"
	"os"
)

// TopicConfig maps a topic name to its threshold/display settings.
// This is the Go equivalent of the SensorTopic entity.
type TopicConfig struct {
	TopicName    string  `json:"topicName"`
	Unit         string  `json:"unit"`
	Color        string  `json:"color"`
	ThresholdMin float64 `json:"thresholdMin"`
	ThresholdMax float64 `json:"thresholdMax"`
}

// SerialPort describes one connected Arduino
type SerialPort struct {
	Port     string `json:"port"`     // e.g. /dev/ttyUSB0 or COM3
	BaudRate int    `json:"baudRate"` // e.g. 9600
	NodeID   string `json:"nodeId"`   // logical ID like "arduino-1"
	Name     string `json:"name"`     // friendly name like "Front Sensor"
}

// Config is the root config struct loaded from config.json
type Config struct {
	DBPath          string        `json:"dbPath"`           // path to SQLite file
	HTTPPort        int           `json:"httpPort"`         // REST + WebSocket port
	SerialPorts     []SerialPort  `json:"serialPorts"`
	Topics          []TopicConfig `json:"topics"`
	NodeDeadTimeout int           `json:"nodeDeadTimeoutSec"` // seconds before NODE_DEATH
	AnomalyCooldown int           `json:"anomalyCooldownSec"` // seconds between same-topic anomalies
	GroqAPIKey      string        `json:"groqApiKey"`         // for AI crash analysis
	GroqModel       string        `json:"groqModel"`          // e.g. llama3-8b-8192
}

// Load reads config.json from the given path
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}

	// Sensible defaults
	if cfg.DBPath == "" {
		cfg.DBPath = "blackbox.db"
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8080
	}
	if cfg.NodeDeadTimeout == 0 {
		cfg.NodeDeadTimeout = 3
	}
	if cfg.AnomalyCooldown == 0 {
		cfg.AnomalyCooldown = 5
	}
	if cfg.GroqModel == "" {
		cfg.GroqModel = "llama3-8b-8192"
	}

	return &cfg, nil
}
