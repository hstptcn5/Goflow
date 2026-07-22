package engine

import (
	"sync"
	"time"
)

type ExecutionEvent struct {
	WorkflowID  string      `json:"workflow_id"`
	ExecutionID string      `json:"execution_id"`
	NodeID      string      `json:"node_id"`
	Status      string      `json:"status"` // 'RUNNING', 'SUCCESS', 'FAILED'
	Timestamp   time.Time   `json:"timestamp"`
	Payload     interface{} `json:"payload,omitempty"`
	Error       string      `json:"error,omitempty"`
	DurationMs  int64       `json:"duration_ms,omitempty"`
}

type EventBus struct {
	subscribers map[chan ExecutionEvent]bool
	mu          sync.RWMutex
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[chan ExecutionEvent]bool),
	}
}

func (eb *EventBus) Subscribe() chan ExecutionEvent {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan ExecutionEvent, 100)
	eb.subscribers[ch] = true
	return ch
}

func (eb *EventBus) Unsubscribe(ch chan ExecutionEvent) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if _, exists := eb.subscribers[ch]; exists {
		delete(eb.subscribers, ch)
		close(ch)
	}
}

func (eb *EventBus) Publish(event ExecutionEvent) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for ch := range eb.subscribers {
		select {
		case ch <- event:
		default:
			// Bỏ qua nếu buffer chan của subscriber bị đầy để tránh làm chậm engine
		}
	}
}
