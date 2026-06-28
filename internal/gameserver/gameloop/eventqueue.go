package gameloop

import (
	"container/heap"
	"time"
)

// Event is executed by the game loop at a scheduled time.
type Event interface {
	ExecuteAt() time.Time
	Execute(gl *GameLoop)
}

// EventQueue is a min-heap of events ordered by execution time.
type EventQueue struct {
	items []Event
}

// Len implements heap.Interface.
func (eq *EventQueue) Len() int { return len(eq.items) }

// Less implements heap.Interface (earliest time = highest priority).
func (eq *EventQueue) Less(i, j int) bool {
	return eq.items[i].ExecuteAt().Before(eq.items[j].ExecuteAt())
}

// Swap implements heap.Interface.
func (eq *EventQueue) Swap(i, j int) {
	eq.items[i], eq.items[j] = eq.items[j], eq.items[i]
}

// Push implements heap.Interface.
func (eq *EventQueue) Push(x interface{}) {
	eq.items = append(eq.items, x.(Event))
}

// Pop implements heap.Interface.
func (eq *EventQueue) Pop() interface{} {
	old := eq.items
	n := len(old)
	item := old[n-1]
	eq.items = old[:n-1]
	return item
}

// Schedule adds an event to the queue.
func (eq *EventQueue) Schedule(e Event) {
	heap.Push(eq, e)
}

// Peek returns the next event without removing it, or nil if empty.
func (eq *EventQueue) Peek() Event {
	if len(eq.items) == 0 {
		return nil
	}
	return eq.items[0]
}

// PopEvent removes and returns the next event, or nil if empty.
func (eq *EventQueue) PopEvent() Event {
	if len(eq.items) == 0 {
		return nil
	}
	return heap.Pop(eq).(Event)
}
