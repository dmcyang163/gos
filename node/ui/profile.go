package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ProfileUI 定义我的界面
type ProfileUI struct {
	Window fyne.Window
	MainUI *MainUI // 添加 MainUI 引用
}

// NewProfileUI 创建并返回一个新的 ProfileUI 实例
func NewProfileUI(window fyne.Window, mainUI *MainUI) *ProfileUI {
	return &ProfileUI{
		Window: window,
		MainUI: mainUI,
	}
}

// Render 渲染我的界面
func (ui *ProfileUI) Render() fyne.CanvasObject {
	// 我的页面内容
	title := widget.NewLabel("我的")
	content := widget.NewLabel("这里是我的界面，显示个人资料。")

	// 设置按钮
	settingsButton := widget.NewButton("设置", func() {
		settingsUI := NewSettingsUI(ui.Window, ui.MainUI)
		ui.MainUI.ContentArea.Objects = []fyne.CanvasObject{settingsUI.Render()}
		ui.MainUI.ContentArea.Refresh()
	})

	// 返回按钮
	backButton := widget.NewButton("返回首页", func() {
		SwitchToHome(ui.MainUI) // 调用 SwitchToHome 切换到首页
	})

	// 返回我的页面布局
	return container.NewVBox(
		title,
		content,
		settingsButton,
		backButton, // 添加返回按钮
	)
}
