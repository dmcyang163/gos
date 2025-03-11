package ui

import (
	"fyne.io/fyne/v2"
)

// 定义当前页面
var currentPage fyne.CanvasObject

// SwitchToHome 切换到首页
func SwitchToHome(mainUI *MainUI) {
	homeUI := NewHomeUI(mainUI.Window)
	currentPage = homeUI.Render()
	mainUI.ContentArea.Objects = []fyne.CanvasObject{currentPage}
	mainUI.ContentArea.Refresh()
}

// SwitchToChat 切换到聊天界面
func SwitchToChat(mainUI *MainUI) {
	chatUI := NewChatUI(mainUI.Window)
	currentPage = chatUI.Render()
	mainUI.ContentArea.Objects = []fyne.CanvasObject{currentPage}
	mainUI.ContentArea.Refresh()
}

// SwitchToContacts 切换到联系人界面
func SwitchToContacts(mainUI *MainUI) {
	contactsUI := NewContactsUI(mainUI.Window)
	currentPage = contactsUI.Render()
	mainUI.ContentArea.Objects = []fyne.CanvasObject{currentPage}
	mainUI.ContentArea.Refresh()
}

// SwitchToProfile 切换到我的界面
func SwitchToProfile(mainUI *MainUI) {
	profileUI := NewProfileUI(mainUI.Window, mainUI)
	currentPage = profileUI.Render()
	mainUI.ContentArea.Objects = []fyne.CanvasObject{currentPage}
	mainUI.ContentArea.Refresh()
}
