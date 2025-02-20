// event.go
package event

type EventType string

type Event struct {
	Type    EventType
	Payload interface{}
}

// EventHandler 是事件处理函数的类型
type EventHandler func(event Event)
