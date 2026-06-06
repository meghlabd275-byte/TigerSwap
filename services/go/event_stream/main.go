package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Event Streaming Platform - Kafka/NATS/Redpanda integration

type Event struct {
	ID        string          `json:"id"`
	Type     string          `json:"type"`
	Topic    string          `json:"topic"`
	Data    json.RawMessage `json:"data"`
	Time    int64          `json:"time"`
}

type Subscriber struct {
	ID      string
	Topics  []string
	Ch     chan Event
}

type EventStream struct {
	mu           sync.RWMutex
	subscribers  map[string]Subscriber
	topics      map[string]int
	eventCount  int64
}

func NewEventStream() *EventStream {
	return &EventStream{
		subscribers: make(map[string]Subscriber),
		topics:     make(map[string]int),
		eventCount: 0,
	}
}

func (e *EventStream) Subscribe(id string, topics []string) chan Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	ch := make(chan Event, 100)
	e.subscribers[id] = Subscriber{
		ID:     id,
		Topics: topics,
		Ch:    ch,
	}
	
	for _, topic := range topics {
		e.topics[topic]++
	}
	
	return ch
}

func (e *EventStream) Unsubscribe(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if sub, ok := e.subscribers[id]; ok {
		close(sub.Ch)
		delete(e.subscribers, id)
		
		for _, topic := range sub.Topics {
			if count, ok := e.topics[topic]; ok {
				e.topics[topic] = count - 1
			}
		}
	}
}

func (e *EventStream) Publish(topic string, eventType string, data interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	eventData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	event := Event{
		ID:     fmt.Sprintf("event-%d", e.eventCount),
		Type:   eventType,
		Topic:  topic,
		Data:  eventData,
		Time:  time.Now().Unix(),
	}
	e.eventCount++
	
	// Fan out to subscribers
	for _, sub := range e.subscribers {
		for _, subTopic := range sub.Topics {
			if subTopic == topic || subTopic == "*" {
				select {
				case sub.Ch <- event:
				default:
					// Channel full, drop
				}
				break
			}
		}
	}
	
	return nil
}

func (e *EventStream) GetTopicCount(topic string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.topics[topic]
}

func (e *EventStream) GetSubscriberCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.subscribers)
}

// Example event types
const (
	EventSwap     = "swap"
	EventOrder   = "order"
	EventTrade  = "trade"
	EventMint   = "mint"
	EventBurn  = "burn"
	EventTransfer = "transfer"
	EventLiquidation = "liquidation"
	EventFunding = "funding"
)

func main() {
	stream := NewEventStream()
	
	// Subscribe to events
	swapCh := stream.Subscribe("swap-bot", []string{EventSwap, EventTrade})
	_ = swapCh
	
	// Publish events
	stream.Publish(EventSwap, EventSwap, map[string]interface{}{
		"token_in": "USDC",
		"token_out": "ETH",
		"amount": 1000,
	})
	
	stream.Publish(EventTrade, EventTrade, map[string]interface{}{
		"pool": "0xPool",
		"from": "0xA",
		"to": "0xB",
		"amount": 100,
	})
	
	fmt.Printf("Subscribers: %d\n", stream.GetSubscriberCount())
	fmt.Printf("Swap events: %d\n", stream.GetTopicCount(EventSwap))
	fmt.Printf("Trade events: %d\n", stream.GetTopicCount(EventTrade))
	
	// Clean up
	stream.Unsubscribe("swap-bot")
}