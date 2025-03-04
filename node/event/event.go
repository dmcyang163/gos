// event.go
package event

type EventType string

const (
	EventTypePeerList        EventType = "peer_list"
	EventTypePeerListRequest EventType = "peer_list_request"
	EventTypeChat            EventType = "chat"
	EventTypePing            EventType = "ping"
	EventTypePong            EventType = "pong"
	EventTypeFileTransfer    EventType = "file_transfer"
	EventTypeNodeStatus      EventType = "node_status"
)

type Event struct {
	Type    EventType
	Payload interface{}
}

// EventHandler 是事件处理函数的类型
type EventHandler func(event Event)
