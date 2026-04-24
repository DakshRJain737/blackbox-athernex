# Blackbox Go Broker

Full-featured message broker for the Blackbox robotics observability platform.

## Quick Start

```bash
# 1. Install dependencies
go mod tidy

# 2. Edit config
nano config/config.json   # set your serial ports and Groq API key

# 3. Run
go run main.go -config config/config.json

# or build a single binary
go build -o blackbox-broker . && ./blackbox-broker
```

## Folder Structure

```
blackbox-broker/
├── main.go              # Entry point — wires everything together
├── config/
│   ├── config.go        # Config struct + loader
│   └── config.json      # Your settings (ports, thresholds, API key)
├── models/
│   └── models.go        # All data types (matches Spring Boot entities)
├── storage/
│   └── storage.go       # SQLite layer — all DB operations
├── pubsub/
│   └── pubsub.go        # In-memory pub/sub bus with last-value cache
├── serial/
│   └── reader.go        # Serial port readers (one goroutine per Arduino)
├── schema/
│   └── validator.go     # Message validation (JSON + plain text fallback)
├── anomaly/
│   └── engine.go        # Threshold breach detection + severity grading
├── watchdog/
│   └── watchdog.go      # Node liveness monitoring
├── ai/
│   └── groq.go          # Groq API client for crash analysis
└── server/
    └── server.go        # HTTP REST API + WebSocket hub
```

## Database Schema

Matches the Spring Boot entities exactly:

| Table            | Spring Boot Entity  |
|------------------|---------------------|
| `sensor_topics`  | `SensorTopic`       |
| `nodes`          | `Node`              |
| `sensor_data`    | `SensorData`        |
| `anomalies`      | `Anomaly`           |
| `crash_events`   | `CrashEvent`        |
| `replay_sessions`| `ReplaySession`     |
| `schema_errors`  | *(new)*             |

## REST API

| Method | Endpoint                     | Description                          |
|--------|------------------------------|--------------------------------------|
| GET    | `/ws`                        | WebSocket — live event stream        |
| GET    | `/stats`                     | System stats (msgs, uptime, nodes)   |
| GET    | `/nodes`                     | All registered nodes                 |
| GET    | `/topics`                    | All sensor topics                    |
| GET    | `/replay?nodeId=&from=&to=`  | Historical sensor data window        |
| GET    | `/sessions`                  | All replay sessions                  |
| GET    | `/anomalies?nodeId=`         | Recent anomalies for a node          |
| GET    | `/crashes?nodeId=&resolved=` | Crash events                         |
| POST   | `/crashes/resolve?id=`       | Mark a crash as resolved             |
| POST   | `/crashes/analyze`           | Trigger AI analysis for a crash      |
| GET    | `/export?nodeId=`            | Export session as CSV                |
| POST   | `/thresholds`                | Update threshold at runtime          |

## WebSocket Events

All events have shape: `{ "type": "EVENT_TYPE", "payload": { ... } }`

| Type           | Payload           | Trigger                              |
|----------------|-------------------|--------------------------------------|
| `MESSAGE`      | `RawMessage`      | Every valid sensor reading           |
| `ANOMALY`      | `Anomaly`         | Threshold breach                     |
| `CRASH`        | `CrashEvent`      | Anomaly + AI analysis result         |
| `NODE_DEATH`   | `NodeStatus`      | Node silent for > nodeDeadTimeoutSec |
| `NODE_ALIVE`   | `NodeStatus`      | Node reconnected                     |
| `SCHEMA_ERROR` | `SchemaError`     | Malformed message received           |
| `SYSTEM_STATS` | `SystemStats`     | Every 2 seconds                      |

## Arduino Sketch (JSON format)

```cpp
#include <ArduinoJson.h>

const String NODE_ID = "arduino-1";

void setup() {
  Serial.begin(9600);
}

void loop() {
  StaticJsonDocument<128> doc;

  // Distance sensor (HC-SR04 or similar)
  float distance = readUltrasonic();
  doc["nodeId"] = NODE_ID;
  doc["topic"]  = "/distance";
  doc["value"]  = distance;
  doc["unit"]   = "cm";
  serializeJson(doc, Serial);
  Serial.println();

  delay(100);

  // Tilt (MPU6050 or analogRead)
  float tilt = readTilt();
  doc["topic"] = "/tilt";
  doc["value"] = tilt;
  doc["unit"]  = "degrees";
  serializeJson(doc, Serial);
  Serial.println();

  delay(100);
}
```

## Arduino Sketch (Plain text fallback)

If ArduinoJson is not available, the broker also accepts:

```
/distance:42.5
/tilt:12.3
/temperature:28.1
```

## Replay Example

```bash
# Get all /distance readings from arduino-1 for the last hour
curl "http://localhost:8080/replay?nodeId=arduino-1&topic=/distance&from=$(date -d '1 hour ago' +%s%3N)&to=$(date +%s%3N)"
```

## AI Crash Analysis

```bash
# Trigger AI analysis for crash event ID 5
curl -X POST http://localhost:8080/crashes/analyze \
  -H "Content-Type: application/json" \
  -d '{"crashId": 5, "nodeId": "arduino-1", "windowSec": 30}'
```

Response:
```json
{
  "rootCause": "Distance sensor reading dropped below 5cm threshold, indicating collision with obstacle at high speed",
  "suggestion": "Add deceleration zone: trigger brake when distance < 15cm, not 5cm",
  "confidence": 0.91
}
```
