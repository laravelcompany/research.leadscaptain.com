package events

import (
	"sync"
	"time"
)

type Event struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}
type Bus struct {
	mu   sync.RWMutex
	subs []chan Event
}

func New() *Bus { return &Bus{} }
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	e.Timestamp = time.Now()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}
func (b *Bus) Subscribe() chan Event {
	ch := make(chan Event, 100)
	b.mu.Lock()
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel so disconnected SSE clients do not leak.
func (b *Bus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, c := range b.subs {
		if c == ch {
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			break
		}
	}
}
