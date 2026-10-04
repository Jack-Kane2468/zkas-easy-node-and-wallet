package main

import (
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"syscall"
)

var subclassDLL = windows.NewLazySystemDLL("comctl32.dll")
var setSubclass = subclassDLL.NewProc("SetWindowSubclass")
var defSubclass = subclassDLL.NewProc("DefSubclassProc")
var removeSubclass = subclassDLL.NewProc("RemoveWindowSubclass")
var wheelProc uintptr

func init() {
	wheelProc = syscall.NewCallback(func(hwnd win.HWND, msg uint32, wp, lp, id, ref uintptr) uintptr {
		if msg == win.WM_MOUSEWHEEL || msg == 0x020E {
			// Forward to the surrounding scroll area, never to the dropdown's selection.
			parent := win.GetParent(hwnd)
			if parent != 0 {
				return win.SendMessage(parent, msg, wp, lp)
			}
			return 0
		}
		if msg == win.WM_NCDESTROY {
			removeSubclass.Call(uintptr(hwnd), wheelProc, id)
		}
		r, _, _ := defSubclass.Call(uintptr(hwnd), uintptr(msg), wp, lp)
		return r
	})
}
func preventWheelChanges(parent win.HWND) {
	cb := syscall.NewCallback(func(hwnd win.HWND, param uintptr) uintptr {
		var b [128]uint16
		n, _ := win.GetClassName(hwnd, &b[0], len(b))
		if n > 0 {
			class := syscall.UTF16ToString(b[:])
			if class == "ComboBox" || class == "ComboBoxEx32" || class == "msctls_updown32" || (class == "Edit" && win.GetWindowLong(hwnd, win.GWL_STYLE)&win.ES_MULTILINE == 0) {
				setSubclass.Call(uintptr(hwnd), wheelProc, 1, 0)
			}
		}
		return 1
	})

	windows.NewLazySystemDLL("user32.dll").NewProc("EnumChildWindows").Call(uintptr(parent), cb, 0)
}
