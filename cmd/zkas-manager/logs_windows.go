package main

import (
	"fmt"
	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"
	"zkas-node-manager/internal/logformat"
)

var logRichLibrary = windows.NewLazySystemDLL("msftedit.dll")

type logView struct {
	hwnd         win.HWND
	text, source string
	force        bool
}

func newLogView(parent *walk.Composite) (*logView, error) {
	if e := logRichLibrary.Load(); e != nil {
		return nil, e
	}
	class, _ := windows.UTF16PtrFromString("RICHEDIT50W")
	h := win.CreateWindowEx(win.WS_EX_CLIENTEDGE, class, nil, win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP|win.WS_VSCROLL|win.WS_HSCROLL|win.ES_MULTILINE|win.ES_READONLY|win.ES_AUTOVSCROLL|win.ES_AUTOHSCROLL, 0, 0, 1, 1, parent.Handle(), 0, win.GetModuleHandle(nil), nil)
	if h == 0 {
		return nil, fmt.Errorf("unable to create log viewer")
	}
	v := &logView{hwnd: h, force: true}
	win.SendMessage(h, win.EM_SETBKGNDCOLOR, 0, uintptr(18|24<<8|34<<16))
	win.SendMessage(h, win.EM_EXLIMITTEXT, 0, 1<<20)
	win.SendMessage(h, win.EM_SETTARGETDEVICE, 0, 1) // horizontal scroll, no wrapping
	resize := func() {
		bottom := v.atBottom()
		r := parent.ClientBoundsPixels()
		win.MoveWindow(h, 0, 0, int32(r.Width), int32(r.Height), true)
		if bottom {
			v.bottom()
		}
	}
	parent.SizeChanged().Attach(resize)
	resize()
	return v, nil
}
func (v *logView) atBottom() bool {
	s := win.SCROLLINFO{CbSize: uint32(unsafe.Sizeof(win.SCROLLINFO{})), FMask: win.SIF_RANGE | win.SIF_PAGE | win.SIF_POS}
	if !win.GetScrollInfo(v.hwnd, win.SB_VERT, &s) || s.NPage == 0 {
		return true
	}
	return int64(s.NPos)+int64(s.NPage) >= int64(s.NMax)-1
}
func (v *logView) selected() bool {
	var s win.CHARRANGE
	win.SendMessage(v.hwnd, win.EM_EXGETSEL, 0, uintptr(unsafe.Pointer(&s)))
	return s.CpMin != s.CpMax
}
func (v *logView) bottom() { win.SendMessage(v.hwnd, win.WM_VSCROLL, win.SB_BOTTOM, 0) }
func (v *logView) show(source, text string, paused bool) {
	changed := source != v.source
	// Keep the displayed snapshot untouched while reading or copying older entries.
	if !changed && !v.force && (paused || !v.atBottom() || v.selected()) {
		return
	}
	text = logformat.Normalize(text)
	if !changed && !v.force && text == v.text {
		return
	}
	var pos win.POINT
	win.SendMessage(v.hwnd, win.EM_GETSCROLLPOS, 0, uintptr(unsafe.Pointer(&pos)))
	if win.IsWindowVisible(v.hwnd) {
		win.SendMessage(v.hwnd, win.WM_SETREDRAW, 0, 0)
		defer func() { win.SendMessage(v.hwnd, win.WM_SETREDRAW, 1, 0); win.InvalidateRect(v.hwnd, nil, false) }()
	}
	rtf := append([]byte(logformat.RTF(text)), 0)
	flags := win.SETTEXTEX{Codepage: 65001}
	win.SendMessage(v.hwnd, win.EM_SETTEXTEX, uintptr(unsafe.Pointer(&flags)), uintptr(unsafe.Pointer(&rtf[0])))
	runtime.KeepAlive(rtf)
	// Clear selection without moving keyboard focus; use the scrollbar to follow the end.
	end := win.CHARRANGE{CpMin: -1, CpMax: -1}
	win.SendMessage(v.hwnd, win.EM_EXSETSEL, 0, uintptr(unsafe.Pointer(&end)))
	v.bottom()
	var current win.POINT
	win.SendMessage(v.hwnd, win.EM_GETSCROLLPOS, 0, uintptr(unsafe.Pointer(&current)))
	if !changed {
		current.X = pos.X
		win.SendMessage(v.hwnd, win.EM_SETSCROLLPOS, 0, uintptr(unsafe.Pointer(&current)))
	}
	v.text = text
	v.source = source
	v.force = false
}
func (m *manager) latestLog() {
	if m.logs == nil {
		return
	}
	m.logPause.SetChecked(false)
	m.logs.force = true
	m.refreshLogs()
}
func (m *manager) refreshLogs() {
	if m.logs == nil {
		return
	}
	names := []string{"console.log", "wallet-console.log", "mining-console.log", "tor-console.log"}
	i := m.logSelect.CurrentIndex()
	if i < 0 || i >= len(names) {
		return
	}
	path := filepath.Join(m.root, "logs", names[i])
	f, e := os.Open(path)
	if e != nil {
		message := "No log entries yet."
		if !os.IsNotExist(e) {
			message = "Unable to read log: " + e.Error()
		}
		m.logs.show(path, message, m.logPause.Checked())
		return
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return
	}
	const limit = 128 * 1024
	offset := st.Size() - limit
	if offset < 0 {
		offset = 0
	}
	if _, e = f.Seek(offset, 0); e != nil {
		return
	}
	b, e := io.ReadAll(io.LimitReader(f, limit))
	if e != nil {
		return
	}
	// Drop a partial first line when reading a bounded tail.
	if offset > 0 {
		for j, c := range b {
			if c == '\n' {
				b = b[j+1:]
				break
			}
		}
	}
	m.logs.show(path, string(b), m.logPause.Checked())
}
