package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// MainUI 定义主 UI 结构
type MainUI struct {
	Window      fyne.Window
	NavBar      *fyne.Container
	ContentArea *fyne.Container // 中部内容区域
}

// NewMainUI 创建并返回一个新的 MainUI 实例
func NewMainUI(window fyne.Window) *MainUI {
	return &MainUI{
		Window: window,
	}
}

// CreateNavBar 创建底部导航栏
func (ui *MainUI) CreateNavBar() *fyne.Container {
	navBar := container.NewHBox(
		widget.NewButton("首页", func() {
			SwitchToHome(ui)
		}),
		widget.NewButton("聊天", func() {
			SwitchToChat(ui)
		}),
		widget.NewButton("联系人", func() {
			SwitchToContacts(ui)
		}),
		widget.NewButton("我的", func() {
			SwitchToProfile(ui)
		}),
	)
	return navBar
}

// Render 渲染主 UI 布局
func (ui *MainUI) Render() fyne.CanvasObject {
	// 创建中部内容区域
	ui.ContentArea = container.NewStack()

	// 初始化首页
	SwitchToHome(ui)

	// 创建底部导航栏
	ui.NavBar = ui.CreateNavBar()

	// 返回主布局
	return container.NewBorder(nil, ui.NavBar, nil, nil, ui.ContentArea)
}
