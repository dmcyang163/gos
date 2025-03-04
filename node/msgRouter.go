// msgRouter.go
package main

import (
	"net"
	"node/utils"
)

// MessageRouter routes messages to the appropriate handler.
type MessageRouter struct {
	handlers map[string]MessageHandler
	logger   utils.Logger
	executor utils.TaskExecutor
}

// NewMessageRouter creates a new MessageRouter instance and registers all handlers.
func NewMessageRouter(logger utils.Logger, executor utils.TaskExecutor) *MessageRouter {
	router := &MessageRouter{
		handlers: make(map[string]MessageHandler),
		logger:   logger,
		executor: executor,
	}

	// 注册所有消息处理器
	router.RegisterHandler(MsgTypePeerList, &PeerListHandler{})
	router.RegisterHandler(MsgTypePeerListReq, &PeerListRequestHandler{})
	router.RegisterHandler(MsgTypeChat, &ChatHandler{})
	router.RegisterHandler(MsgTypePing, &PingHandler{})
	router.RegisterHandler(MsgTypePong, &PongHandler{})
	router.RegisterHandler(MsgTypeFileTransfer, &FileTransferHandler{})
	router.RegisterHandler(MsgTypeNodeStatus, &NodeStatusHandler{})

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
		r.logger.Warnf("Unknown message type: %s", msg.Type)
		return
	}
	handler.HandleMessage(n, conn, msg)
}
