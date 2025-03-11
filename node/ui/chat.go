package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ChatUI 定义聊天界面
type ChatUI struct {
	Window       fyne.Window
	MessageEntry *widget.Entry
	SendButton   *widget.Button
	MessageList  *widget.List
	Messages     []string
}

// NewChatUI 创建并返回一个新的 ChatUI 实例
func NewChatUI(window fyne.Window) *ChatUI {
	ui := &ChatUI{
		Window:   window,
		Messages: []string{},
	}
	// 设置窗口的初始大小
	window.Resize(fyne.NewSize(400, 600)) // 设置窗口大小为 400x600

	ui.MessageEntry = widget.NewEntry()
	ui.MessageEntry.SetPlaceHolder("输入消息...")

	ui.SendButton = widget.NewButton("发送", ui.sendMessage)

	ui.MessageList = widget.NewList(
		func() int {
			return len(ui.Messages)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(ui.Messages[i])
		},
	)

	return ui
}

// Render 渲染聊天界面
func (ui *ChatUI) Render() fyne.CanvasObject {
	title := widget.NewLabel("聊天")

	// 创建消息输入和发送按钮的容器
	inputContainer := container.NewBorder(nil, nil, nil, ui.SendButton, ui.MessageEntry)

	// 创建主界面布局
	mainContainer := container.NewBorder(
		title,
		inputContainer,
		nil,
		nil,
		ui.MessageList,
	)

	return mainContainer
}

// sendMessage 处理发送消息的逻辑
func (ui *ChatUI) sendMessage() {
	message := ui.MessageEntry.Text
	if message != "" {
		ui.Messages = append(ui.Messages, message)
		ui.MessageList.Refresh()
		ui.MessageEntry.SetText("")
	}
}
