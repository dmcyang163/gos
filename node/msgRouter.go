// msgRouter.go

package main

import (
	"net"
	"node/utils"
)

// MessageRouter 将消息路由到适当的处理程序。
type MessageRouter struct {
	handlers map[string]MessageHandler
	logger   utils.Logger
}

// NewMessageRouter 创建一个新的 MessageRouter 实例并注册所有处理程序。
func NewMessageRouter(logger utils.Logger) *MessageRouter {
	router := &MessageRouter{
		handlers: make(map[string]MessageHandler),
		logger:   logger,
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

// RegisterHandler 注册特定消息类型的处理程序。
func (r *MessageRouter) RegisterHandler(msgType string, handler MessageHandler) {
	r.handlers[msgType] = handler
}

// RouteMessage 将消息路由到适当的处理程序。
func (r *MessageRouter) RouteMessage(n *Node, conn net.Conn, msg Message) {
	handler, ok := r.handlers[msg.Type]
	if !ok {
		r.logger.Warnf("Unknown message type: %s", msg.Type)
		return
	}
	handler.HandleMessage(n, conn, msg)
}
