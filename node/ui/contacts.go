package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ContactsUI 定义联系人界面
type ContactsUI struct {
	Window fyne.Window
}

// NewContactsUI 创建并返回一个新的 ContactsUI 实例
func NewContactsUI(window fyne.Window) *ContactsUI {
	return &ContactsUI{
		Window: window,
	}
}

// Render 渲染联系人界面
func (ui *ContactsUI) Render() fyne.CanvasObject {
	title := widget.NewLabel("联系人")
	content := widget.NewLabel("这里是联系人界面，显示联系人列表。")
	return container.NewVBox(title, content)
}
