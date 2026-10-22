package main

import (
	"fmt"
	"github.com/lxn/win"
	"syscall"
)

// ComboBox notifications go to its parent. Defer layout-changing wallet renders
// until Windows has finished closing the list, and ignore canceled selections.
func (m *manager) trackWalletPicker() error {
	picker := m.walletPicker.Handle()
	parent := win.GetParent(picker)
	m.walletPickerProc = syscall.NewCallback(func(hwnd win.HWND, msg uint32, wp, lp, id, ref uintptr) uintptr {
		closeList := false
		if msg == win.WM_COMMAND && win.HWND(lp) == picker {
			switch win.HIWORD(uint32(wp)) {
			case win.CBN_DROPDOWN:
				m.walletPickerOpen = true
				m.walletPickerAccepted = false
				m.walletPickerStart = m.walletPicker.CurrentIndex()
			case win.CBN_SELENDOK:
				m.walletPickerAccepted = true
			case win.CBN_SELENDCANCEL:
				m.walletPickerAccepted = false
			case win.CBN_CLOSEUP:
				closeList = true
			}
		}
		if msg == win.WM_NCDESTROY {
			removeSubclass.Call(uintptr(hwnd), m.walletPickerProc, id)
		}
		result, _, _ := defSubclass.Call(uintptr(hwnd), uintptr(msg), wp, lp)
		if closeList {
			accepted, start := m.walletPickerAccepted, m.walletPickerStart
			m.window.Synchronize(func() {
				if m.closed.Load() {
					return
				}
				if !accepted {
					m.walletPickerChanging = true
					m.walletPicker.SetCurrentIndex(start)
					m.walletPickerChanging = false
				}
				m.walletPickerOpen = false
				if accepted {
					m.chooseWallet()
				}
			})
		}
		return result
	})
	ok, _, err := setSubclass.Call(uintptr(parent), m.walletPickerProc, 2, 0)
	if ok == 0 {
		return fmt.Errorf("Cannot initialize wallet selector: %v", err)
	}
	return nil
}
