package main

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// Keep troubleshooting details in the error dialog, not in permanent wallet controls.
func (m *manager) showOperationError(title string, cause error) {
	var d *walk.Dialog
	var closeButton *walk.PushButton
	err := (Dialog{
		AssignTo: &d, Title: title, Size: Size{Width: 600, Height: 320},
		Layout: VBox{}, DefaultButton: &closeButton, CancelButton: &closeButton,
		Children: []Widget{
			TextLabel{Text: "The operation could not complete.", Font: Font{Bold: true}},
			TextEdit{ReadOnly: true, VScroll: true, Text: cause.Error(), MinSize: Size{Height: 120}},
			Composite{Layout: HBox{}, Children: []Widget{
				PushButton{Text: "Copy details", OnClicked: func() { m.copyText("Error details", cause.Error()) }},
				HSpacer{},
				PushButton{AssignTo: &closeButton, Text: "Close", OnClicked: func() { d.Accept() }},
			}},
		},
	}).Create(m.window)
	if d != nil {
		defer d.Dispose()
	}
	if err != nil {
		walk.MsgBox(m.window, title, cause.Error(), walk.MsgBoxIconError)
		return
	}
	d.Run()
}
