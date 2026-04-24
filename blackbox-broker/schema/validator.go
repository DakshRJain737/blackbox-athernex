package schema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blackbox/broker/models"
)

// Validate parses and validates a raw byte payload from a serial port.
// Returns a populated RawMessage on success, or a SchemaError on failure.
// The nodeID is the logical Arduino id from config (used for error logging).
func Validate(raw []byte, configNodeID string, ts int64) (*models.RawMessage, *models.SchemaError) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, &models.SchemaError{
			RawPayload:  trimmed,
			ErrorReason: "empty message",
			Timestamp:   ts,
			NodeID:      configNodeID,
		}
	}

	// ── Try JSON first ───────────────────────────────────────────────────────
	var msg models.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &msg); err != nil {
		// ── Fallback: accept "topic:value" plain text ────────────────────────
		parsed, pErr := parsePlainText(trimmed, configNodeID)
		if pErr != nil {
			return nil, &models.SchemaError{
				RawPayload:  trimmed,
				ErrorReason: fmt.Sprintf("invalid JSON: %v", err),
				Timestamp:   ts,
				NodeID:      configNodeID,
			}
		}
		return parsed, nil
	}

	// Required fields
	if msg.Topic == "" {
		return nil, schemaErr(trimmed, "missing field: topic", ts, configNodeID)
	}
	if msg.NodeID == "" {
		msg.NodeID = configNodeID // default to the port-level nodeId
	}

	// Normalise topic — always starts with /
	if !strings.HasPrefix(msg.Topic, "/") {
		msg.Topic = "/" + msg.Topic
	}

	return &msg, nil
}

// parsePlainText handles simple "topic:value" messages from basic Arduino sketches.
// Example: "/distance:42.5"  or  "tilt:12"
func parsePlainText(s, nodeID string) (*models.RawMessage, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("not plain text format")
	}
	topic := strings.TrimSpace(parts[0])
	valueStr := strings.TrimSpace(parts[1])

	var value float64
	if _, err := fmt.Sscanf(valueStr, "%f", &value); err != nil {
		return nil, fmt.Errorf("non-numeric value: %s", valueStr)
	}
	if !strings.HasPrefix(topic, "/") {
		topic = "/" + topic
	}
	return &models.RawMessage{
		NodeID: nodeID,
		Topic:  topic,
		Value:  value,
	}, nil
}

func schemaErr(raw, reason string, ts int64, nodeID string) *models.SchemaError {
	return &models.SchemaError{
		RawPayload:  raw,
		ErrorReason: reason,
		Timestamp:   ts,
		NodeID:      nodeID,
	}
}
