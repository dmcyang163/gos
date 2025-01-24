package main

import "net"

// MessageRouter routes messages to the appropriate handler.
type MessageRouter struct {
	handlers map[string]MessageHandler
}

// NewMessageRouter creates a new MessageRouter instance and registers all handlers.
func NewMessageRouter() *MessageRouter {
	router := &MessageRouter{
		handlers: make(map[string]MessageHandler),
	}

	// 注册所有消息处理器
	router.RegisterHandler(MessageTypePeerList, &PeerListHandler{})
	router.RegisterHandler(MessageTypePeerListReq, &PeerListRequestHandler{})
	router.RegisterHandler(MessageTypeChat, &ChatHandler{})
	router.RegisterHandler(MessageTypePing, &PingHandler{})
	router.RegisterHandler(MessageTypePong, &PongHandler{})
	router.RegisterHandler(MessageTypeFileTransfer, &FileTransferHandler{})
	router.RegisterHandler(MessageTypeNodeStatus, &NodeStatusHandler{})

	return router
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
