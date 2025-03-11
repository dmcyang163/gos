package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// Contact 定义联系人结构体
type Contact struct {
	Name  string
	Phone string
	Email string
}

// ContactsUI 定义联系人界面
type ContactsUI struct {
	Window      fyne.Window
	ContactList *widget.List
	Contacts    []Contact
	Selected    int // 用于跟踪当前选中的联系人索引
}

// NewContactsUI 创建并返回一个新的 ContactsUI 实例
func NewContactsUI(window fyne.Window) *ContactsUI {
	ui := &ContactsUI{
		Window:   window,
		Contacts: []Contact{},
		Selected: -1, // 初始化为 -1，表示没有选中任何项
	}

	// 设置窗口的初始大小
	window.Resize(fyne.NewSize(400, 600))

	// 创建联系人列表
	ui.ContactList = widget.NewList(
		func() int {
			return len(ui.Contacts)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			contact := ui.Contacts[i]
			o.(*widget.Label).SetText(contact.Name + " - " + contact.Phone)
		},
	)

	// 设置列表项选中时的回调函数
	ui.ContactList.OnSelected = func(id widget.ListItemID) {
		ui.Selected = id // 更新选中的索引
	}

	return ui
}

// Render 渲染联系人界面
func (ui *ContactsUI) Render() fyne.CanvasObject {
	title := widget.NewLabel("联系人")

	// 创建添加联系人按钮
	addButton := widget.NewButton("添加联系人", ui.showAddContactDialog)

	// 创建删除联系人按钮
	deleteButton := widget.NewButton("删除联系人", ui.deleteContact)

	// 创建联系人列表的容器
	listContainer := container.NewBorder(nil, nil, nil, nil, ui.ContactList)

	// 创建主界面布局
	mainContainer := container.NewBorder(
		container.NewVBox(title, addButton, deleteButton),
		nil,
		nil,
		nil,
		listContainer,
	)

	return mainContainer
}

// showAddContactDialog 显示添加联系人的对话框
func (ui *ContactsUI) showAddContactDialog() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("输入联系人姓名")
	nameEntry.Resize(fyne.NewSize(200, 30)) // 设置最小宽度为 200 像素

	phoneEntry := widget.NewEntry()
	phoneEntry.SetPlaceHolder("输入联系人电话号码")
	phoneEntry.Resize(fyne.NewSize(200, 30)) // 设置最小宽度为 200 像素

	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("输入联系人电子邮件")
	emailEntry.Resize(fyne.NewSize(200, 30)) // 设置最小宽度为 200 像素

	// 使用 HBox 将标签和输入框放在同一行
	nameRow := container.NewHBox(
		widget.NewLabel("姓名:"),
		nameEntry, // 确保输入框足够宽
	)

	phoneRow := container.NewHBox(
		widget.NewLabel("电话号码:"),
		phoneEntry, // 确保输入框足够宽
	)

	emailRow := container.NewHBox(
		widget.NewLabel("电子邮件:"),
		emailEntry, // 确保输入框足够宽
	)

	// 使用 VBox 布局所有行
	formItems := container.NewVBox(
		nameRow,
		phoneRow,
		emailRow,
	)

	// 创建一个固定宽度的容器，确保弹窗宽度至少为 400 像素
	formContainer := container.NewBorder(
		nil,
		nil,
		nil,
		nil,
		container.NewVBox(formItems),
	)
	formContainer.Resize(fyne.NewSize(400, 300)) // 设置弹窗的最小宽度为 400 像素

	// 显示对话框
	dialog.ShowCustomConfirm("添加联系人", "确认", "取消", formContainer, func(confirmed bool) {
		if confirmed {
			name := nameEntry.Text
			phone := phoneEntry.Text
			email := emailEntry.Text

			if name != "" {
				// 添加新联系人
				newContact := Contact{
					Name:  name,
					Phone: phone,
					Email: email,
				}
				ui.Contacts = append(ui.Contacts, newContact)
				ui.ContactList.Refresh()
			}
		}
	}, ui.Window)
}

// deleteContact 删除选中的联系人
func (ui *ContactsUI) deleteContact() {
	if ui.Selected >= 0 && ui.Selected < len(ui.Contacts) {
		ui.Contacts = append(ui.Contacts[:ui.Selected], ui.Contacts[ui.Selected+1:]...)
		ui.ContactList.Refresh()
		ui.Selected = -1 // 重置选中的索引
	}
}
