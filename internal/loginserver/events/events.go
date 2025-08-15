package events

import (
	"context"
	"sync"
)

// Event represents a system event
type Event interface {
	Type() string
}

// EventHandler handles specific event types
type EventHandler func(ctx context.Context, event Event) error

// EventBus manages event publishing and subscription
type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(eventType string, handler EventHandler)
	Unsubscribe(eventType string, handler EventHandler)
}

// InMemoryEventBus provides in-memory event bus implementation
type InMemoryEventBus struct {
	handlers map[string][]EventHandler
	mu       sync.RWMutex
}

func NewEventBus() EventBus {
	return &InMemoryEventBus{
		handlers: make(map[string][]EventHandler),
	}
}

func (bus *InMemoryEventBus) Publish(ctx context.Context, event Event) error {
	bus.mu.RLock()
	handlers, exists := bus.handlers[event.Type()]
	if !exists {
		bus.mu.RUnlock()
		return nil // No handlers for this event type
	}

	// Copy handlers to avoid holding lock during execution
	handlersCopy := make([]EventHandler, len(handlers))
	copy(handlersCopy, handlers)
	bus.mu.RUnlock()

	// Execute handlers concurrently
	for _, handler := range handlersCopy {
		// In production, might want to handle errors differently
		go handler(ctx, event)
	}

	return nil
}

func (bus *InMemoryEventBus) Subscribe(eventType string, handler EventHandler) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	bus.handlers[eventType] = append(bus.handlers[eventType], handler)
}

func (bus *InMemoryEventBus) Unsubscribe(eventType string, handler EventHandler) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	handlers := bus.handlers[eventType]
	for i, h := range handlers {
		// Function comparison is tricky, this is simplified
		if &h == &handler {
			bus.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
}

// RequestCharactersEvent - событие для запроса character counts
type RequestCharactersEvent struct {
	Account  string
	ServerID int
}

func (e *RequestCharactersEvent) Type() string {
	return "request_characters"
}

// SendPacketEvent - событие для отправки пакета GameServer'у
type SendPacketEvent struct {
	ServerID int
	Data     []byte
}

func (e *SendPacketEvent) Type() string {
	return "send_packet"
}
