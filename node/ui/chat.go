package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ChatUI 定义聊天界面
type ChatUI struct {
	Window fyne.Window
}

// NewChatUI 创建并返回一个新的 ChatUI 实例
func NewChatUI(window fyne.Window) *ChatUI {
	return &ChatUI{
		Window: window,
	}
}

// Render 渲染聊天界面
func (ui *ChatUI) Render() fyne.CanvasObject {
	title := widget.NewLabel("聊天")
	content := widget.NewLabel("这里是聊天界面，显示聊天记录。")
	return container.NewVBox(title, content)
}
