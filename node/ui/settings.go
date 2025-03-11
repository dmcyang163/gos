package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// SettingsUI 定义设置界面
type SettingsUI struct {
	Window fyne.Window
	MainUI *MainUI // 添加 MainUI 引用
}

// NewSettingsUI 创建并返回一个新的 SettingsUI 实例
func NewSettingsUI(window fyne.Window, mainUI *MainUI) *SettingsUI {
	return &SettingsUI{
		Window: window,
		MainUI: mainUI,
	}
}

// Render 渲染设置界面
func (ui *SettingsUI) Render() fyne.CanvasObject {
	// 返回按钮
	backButton := widget.NewButton("返回", func() {
		profileUI := NewProfileUI(ui.Window, ui.MainUI)
		ui.MainUI.ContentArea.Objects = []fyne.CanvasObject{profileUI.Render()}
		ui.MainUI.ContentArea.Refresh()
	})

	// 主题切换按钮
	themeButton := widget.NewButton("切换主题", func() {
		currentTheme := fyne.CurrentApp().Settings().Theme()
		// 通过比较背景颜色来判断当前主题
		if currentTheme.Color(theme.ColorNameBackground, theme.VariantLight) == theme.LightTheme().Color(theme.ColorNameBackground, theme.VariantLight) {
			fyne.CurrentApp().Settings().SetTheme(theme.DarkTheme())
		} else {
			fyne.CurrentApp().Settings().SetTheme(theme.LightTheme())
		}
		ui.MainUI.Window.Content().Refresh() // 刷新窗口内容
	})

	// 设置页面布局
	return container.NewVBox(
		backButton,
		widget.NewLabel("设置"),
		themeButton,
	)
}
