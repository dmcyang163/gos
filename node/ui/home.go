package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// HomeUI 定义首页界面
type HomeUI struct {
	Window fyne.Window
}

// NewHomeUI 创建并返回一个新的 HomeUI 实例
func NewHomeUI(window fyne.Window) *HomeUI {
	return &HomeUI{
		Window: window,
	}
}

// Render 渲染首页界面
func (ui *HomeUI) Render() fyne.CanvasObject {
	title := widget.NewLabel("首页")
	content := widget.NewLabel("这里是首页内容，显示动态信息。")
	return container.NewVBox(title, content)
}
