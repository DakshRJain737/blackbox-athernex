package watchdog

import (
	"sync"
	"time"

	"github.com/blackbox/broker/models"
	"github.com/blackbox/broker/pubsub"
)

type nodeState struct {
	lastSeen      time.Time
	totalMessages int64
	alive         bool
	name          string
}

// Watchdog monitors every known node.
// It ticks every second and publishes NODE_DEATH if a node goes silent,
// and NODE_ALIVE when it comes back.
type Watchdog struct {
	mu      sync.Mutex
	nodes   map[string]*nodeState // nodeId → state
	timeout time.Duration
	bus     *pubsub.Bus
}

// New creates a Watchdog with the given dead-timeout.
func New(timeoutSec int, bus *pubsub.Bus) *Watchdog {
	return &Watchdog{
		nodes:   make(map[string]*nodeState),
		timeout: time.Duration(timeoutSec) * time.Second,
		bus:     bus,
	}
}

// Beat records a heartbeat for a node (called every time a message arrives).
func (w *Watchdog) Beat(nodeID, name string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	state, ok := w.nodes[nodeID]
	if !ok {
		state = &nodeState{alive: true, name: name}
		w.nodes[nodeID] = state
	}
	state.lastSeen = time.Now()
	state.totalMessages++
	if !state.alive {
		state.alive = true
		// Fire NODE_ALIVE
		w.bus.Publish(models.WSEvent{
			Type: models.EventNodeAlive,
			Payload: models.NodeStatus{
				NodeID:        nodeID,
				IsAlive:       true,
				LastSeen:      time.Now().UnixMilli(),
				TotalMessages: state.totalMessages,
				Status:        "ALIVE",
			},
		})
	}
}

// Start runs the watchdog loop in its own goroutine.
func (w *Watchdog) Start() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			w.tick()
		}
	}()
}

func (w *Watchdog) tick() {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	for nodeID, state := range w.nodes {
		if state.alive && now.Sub(state.lastSeen) > w.timeout {
			state.alive = false
			w.bus.Publish(models.WSEvent{
				Type: models.EventNodeDeath,
				Payload: models.NodeStatus{
					NodeID:        nodeID,
					IsAlive:       false,
					LastSeen:      state.lastSeen.UnixMilli(),
					TotalMessages: state.totalMessages,
					Status:        "DEAD",
				},
			})
		}
	}
}

// Statuses returns current NodeStatus for all known nodes.
func (w *Watchdog) Statuses() []models.NodeStatus {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := make([]models.NodeStatus, 0, len(w.nodes))
	for nodeID, state := range w.nodes {
		status := "ALIVE"
		if !state.alive {
			status = "DEAD"
		}
		out = append(out, models.NodeStatus{
			NodeID:        nodeID,
			IsAlive:       state.alive,
			LastSeen:      state.lastSeen.UnixMilli(),
			TotalMessages: state.totalMessages,
			Status:        status,
		})
	}
	return out
}

// ActiveCount returns the number of alive nodes.
func (w *Watchdog) ActiveCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, s := range w.nodes {
		if s.alive {
			n++
		}
	}
	return n
}
