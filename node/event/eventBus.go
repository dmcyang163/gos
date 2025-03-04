// eventBus.go
package event

import "sync"

var eventBus = NewEventBus()

func GetEventBus() *EventBus {
	return eventBus
}

type EventBus struct {
	subscribers map[EventType][]EventHandler
	lock        sync.RWMutex
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]EventHandler),
	}
}

// Subscribe 订阅事件
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	eb.lock.Lock()
	defer eb.lock.Unlock()
	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)
}

// Publish 发布事件
func (eb *EventBus) Publish(event Event) {
	eb.lock.RLock()
	defer eb.lock.RUnlock()
	if handlers, ok := eb.subscribers[event.Type]; ok {
		for _, handler := range handlers {
			go handler(event) // 异步处理事件
		}
	}
}
