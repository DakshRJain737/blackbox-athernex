package anomaly

import (
	"sync"
	"time"

	"github.com/blackbox/broker/models"
)

// TopicThreshold holds the min/max for one topic.
type TopicThreshold struct {
	Min float64
	Max float64
}

// Engine checks every sensor reading against configured thresholds.
// It rate-limits alerts: the same topic won't fire again within CooldownSec.
type Engine struct {
	mu          sync.Mutex
	thresholds  map[string]TopicThreshold // topic → thresholds
	lastFired   map[string]time.Time      // topic → time of last anomaly
	cooldown    time.Duration
}

// New creates an Engine using the given topic configs.
func New(topics []models.SensorTopic, cooldownSec int) *Engine {
	e := &Engine{
		thresholds: make(map[string]TopicThreshold),
		lastFired:  make(map[string]time.Time),
		cooldown:   time.Duration(cooldownSec) * time.Second,
	}
	for _, t := range topics {
		e.thresholds[t.TopicName] = TopicThreshold{Min: t.ThresholdMin, Max: t.ThresholdMax}
	}
	return e
}

// CheckResult is returned by Check.
type CheckResult struct {
	IsAnomaly bool
	Threshold float64  // the threshold that was crossed
	Severity  models.Severity
}

// Check tests a single sensor value.
// Returns a CheckResult; IsAnomaly==true means the caller should persist an
// Anomaly + CrashEvent and broadcast an alert.
func (e *Engine) Check(topic string, value float64) CheckResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	th, ok := e.thresholds[topic]
	if !ok {
		return CheckResult{} // unknown topic — no threshold configured
	}

	var violated bool
	var threshold float64

	if value < th.Min {
		violated = true
		threshold = th.Min
	} else if value > th.Max {
		violated = true
		threshold = th.Max
	}

	if !violated {
		return CheckResult{}
	}

	// Rate-limit: don't re-fire within cooldown window
	if last, seen := e.lastFired[topic]; seen && time.Since(last) < e.cooldown {
		return CheckResult{}
	}
	e.lastFired[topic] = time.Now()

	return CheckResult{
		IsAnomaly: true,
		Threshold: threshold,
		Severity:  computeSeverity(value, th),
	}
}

// UpdateThreshold lets the REST API adjust a threshold at runtime.
func (e *Engine) UpdateThreshold(topic string, min, max float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.thresholds[topic] = TopicThreshold{Min: min, Max: max}
}

// computeSeverity grades the breach: the farther outside the band, the higher.
func computeSeverity(value float64, th TopicThreshold) models.Severity {
	bandSize := th.Max - th.Min
	if bandSize <= 0 {
		return models.SeverityHigh
	}
	var deviation float64
	if value < th.Min {
		deviation = (th.Min - value) / bandSize
	} else {
		deviation = (value - th.Max) / bandSize
	}
	switch {
	case deviation > 1.0:
		return models.SeverityCritical
	case deviation > 0.5:
		return models.SeverityHigh
	case deviation > 0.2:
		return models.SeverityMedium
	default:
		return models.SeverityLow
	}
}
