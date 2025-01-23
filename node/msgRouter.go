package main

import "net"

// MessageRouter routes messages to the appropriate handler.
type MessageRouter struct {
	handlers map[string]MessageHandler
}

// NewMessageRouter creates a new MessageRouter instance.
func NewMessageRouter() *MessageRouter {
	return &MessageRouter{
		handlers: make(map[string]MessageHandler),
	}
}

// RegisterHandler registers a handler for a specific message type.
func (r *MessageRouter) RegisterHandler(msgType string, handler MessageHandler) {
	r.handlers[msgType] = handler
}

// RouteMessage routes a message to the appropriate handler.
func (r *MessageRouter) RouteMessage(n *Node, conn net.Conn, msg Message) {
	handler, ok := r.handlers[msg.Type]
	if !ok {
		n.logger.Warnf("Unknown message type: %s", msg.Type)
		return
	}
	handler.HandleMessage(n, conn, msg)
}
