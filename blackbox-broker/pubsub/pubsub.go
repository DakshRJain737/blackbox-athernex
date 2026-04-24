package pubsub

import (
	"sync"

	"github.com/blackbox/broker/models"
)

// Subscriber receives WebSocket events.
type Subscriber struct {
	ID   string
	Chan chan models.WSEvent
}

// Bus is a simple in-memory pub/sub engine.
// - Any goroutine can Publish() to a topic.
// - WebSocket handlers Subscribe() and receive every event on their channel.
// - New subscribers immediately receive the last known value for each topic (LVC).
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string]*Subscriber        // subscriber id → subscriber
	lastValue   map[string]models.WSEvent     // topic → last event (LVC)
}

// New creates an initialised Bus.
func New() *Bus {
	return &Bus{
		subscribers: make(map[string]*Subscriber),
		lastValue:   make(map[string]models.WSEvent),
	}
}

// Subscribe registers a new subscriber and returns it.
// The subscriber's channel is buffered to avoid blocking publishers.
// The caller is responsible for calling Unsubscribe when done.
func (b *Bus) Subscribe(id string) *Subscriber {
	sub := &Subscriber{
		ID:   id,
		Chan: make(chan models.WSEvent, 256),
	}
	b.mu.Lock()
	b.subscribers[id] = sub
	// Send all last-known values immediately so the new client is up to date
	lvc := make([]models.WSEvent, 0, len(b.lastValue))
	for _, ev := range b.lastValue {
		lvc = append(lvc, ev)
	}
	b.mu.Unlock()

	for _, ev := range lvc {
		select {
		case sub.Chan <- ev:
		default:
		}
	}
	return sub
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *Bus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sub, ok := b.subscribers[id]; ok {
		close(sub.Chan)
		delete(b.subscribers, id)
	}
}

// Publish broadcasts an event to all current subscribers.
// If the event type is MESSAGE, it also updates the last-value cache keyed
// by "<nodeId>:<topic>" so new subscribers can catch up.
func (b *Bus) Publish(ev models.WSEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Update LVC for sensor messages
	if ev.Type == models.EventMessage {
		if msg, ok := ev.Payload.(models.RawMessage); ok {
			key := msg.NodeID + ":" + msg.Topic
			b.lastValue[key] = ev
		}
	}

	// Fan-out — non-blocking send; slow subscribers drop messages
	for _, sub := range b.subscribers {
		select {
		case sub.Chan <- ev:
		default:
		}
	}
}

// SubscriberCount returns the current number of connected subscribers.
func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
