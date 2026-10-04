package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

// copyText runs on the UI thread, using the manager as clipboard owner.
func (m *manager) copyText(label, text string) {
	if err := setClipboardText(m.window.Handle(), text); err != nil {
		m.progress.SetText("Copy failed. Please try again.")
		walk.MsgBox(m.window, "Unable to copy", fmt.Sprintf("%s could not be copied.\n\n%v\n\nClose any clipboard dialog in another app and try again.", label, err), walk.MsgBoxIconWarning)
		return
	}
	m.progress.SetText(label + " copied. Paste with Ctrl+V.")
}

func setClipboardText(owner win.HWND, text string) error {
	value, err := syscall.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("encode clipboard text: %w", err)
	}
	// Allocate before clearing the existing clipboard; Windows takes ownership
	// only after SetClipboardData succeeds.
	mem := win.GlobalAlloc(win.GMEM_MOVEABLE, uintptr(len(value)*2))
	if mem == 0 {
		return fmt.Errorf("unable to allocate clipboard memory")
	}
	transferred := false
	defer func() {
		if !transferred {
			win.GlobalFree(mem)
		}
	}()
	ptr := win.GlobalLock(mem)
	if ptr == nil {
		return fmt.Errorf("unable to lock clipboard memory")
	}
	win.MoveMemory(ptr, unsafe.Pointer(&value[0]), uintptr(len(value)*2))
	win.GlobalUnlock(mem)
	opened := false
	for attempt := 0; attempt < 8; attempt++ {
		if win.OpenClipboard(owner) {
			opened = true
			break
		}
		if attempt < 7 {
			time.Sleep(25 * time.Millisecond)
		}
	}
	if !opened {
		return fmt.Errorf("Windows clipboard is busy or unavailable")
	}
	defer win.CloseClipboard()
	// Required: EmptyClipboard assigns ownership to the OpenClipboard window.
	// Opening and setting data alone can fail when another app owns the clipboard.
	if !win.EmptyClipboard() {
		return fmt.Errorf("Windows could not clear the clipboard")
	}
	if win.SetClipboardData(win.CF_UNICODETEXT, win.HANDLE(mem)) == 0 {
		return fmt.Errorf("Windows could not store the clipboard text")
	}
	transferred = true
	return nil
}
