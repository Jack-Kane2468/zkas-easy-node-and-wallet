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
	"strings"
	"syscall"
	"unsafe"
	"zkas-node-manager/internal/logformat"
	"zkas-node-manager/internal/node"
)

var logRichLibrary = windows.NewLazySystemDLL("msftedit.dll")

type logAction struct {
	position int32
	cursor   string
}
type logButton struct {
	button win.HWND
	cursor string
	token  string
}
type logView struct {
	parent       *walk.Composite
	rescan       func(string, string)
	owners       map[string][]string
	actions      []logAction
	buttons      []logButton
	scrollProc   uintptr
	hwnd         win.HWND
	text, source string
	force        bool
	fileInfo     os.FileInfo
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
	v := &logView{hwnd: h, force: true, parent: parent}
	v.scrollProc = syscall.NewCallback(func(hwnd win.HWND, msg uint32, wp, lp, id, ref uintptr) uintptr {
		if msg == win.WM_COMMAND && win.HIWORD(uint32(wp)) == win.BN_CLICKED {
			for _, b := range v.buttons {
				if b.button == win.HWND(lp) && b.cursor != "" {
					if v.rescan != nil {
						v.rescan(b.cursor, b.token)
					}
					return 0
				}
			}
		}
		if msg == win.WM_NCDESTROY {
			removeSubclass.Call(uintptr(hwnd), v.scrollProc, id)
		}
		result, _, _ := defSubclass.Call(uintptr(hwnd), uintptr(msg), wp, lp)
		if msg == win.WM_VSCROLL || msg == win.WM_HSCROLL || msg == win.WM_MOUSEWHEEL || msg == win.WM_KEYUP {
			v.layoutActions()
		}
		return result
	})
	ok, _, err := setSubclass.Call(uintptr(h), v.scrollProc, 3, 0)
	if ok == 0 {
		return nil, fmt.Errorf("cannot initialize log actions: %v", err)
	}
	win.SendMessage(h, win.EM_SETBKGNDCOLOR, 0, uintptr(18|24<<8|34<<16))
	win.SendMessage(h, win.EM_EXLIMITTEXT, 0, 256<<10)
	win.SendMessage(h, win.EM_SETUNDOLIMIT, 0, 0)
	font := win.CHARFORMAT{CbSize: uint32(unsafe.Sizeof(win.CHARFORMAT{})), DwMask: win.CFM_FACE | win.CFM_SIZE | win.CFM_COLOR, YHeight: 200, CrTextColor: logColors[1]}
	copy(font.SzFaceName[:], windows.StringToUTF16("Consolas"))
	win.SendMessage(h, win.EM_SETCHARFORMAT, win.SCF_ALL, uintptr(unsafe.Pointer(&font)))
	win.SendMessage(h, win.EM_SETTARGETDEVICE, 0, 1) // horizontal scroll, no wrapping
	resize := func() {
		bottom := v.atBottom()
		r := parent.ClientBoundsPixels()
		win.MoveWindow(h, 0, 0, int32(r.Width), int32(r.Height), true)
		if bottom {
			v.bottom()
		}
		v.layoutActions()
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
	var actions []logAction
	if filepath.Base(source) == "wallet-console.log" {
		text, actions = decorateCheckpointLog(text)
	}
	if !changed && !v.force && text == v.text {
		return
	}
	var pos win.POINT
	win.SendMessage(v.hwnd, win.EM_GETSCROLLPOS, 0, uintptr(unsafe.Pointer(&pos)))
	if win.IsWindowVisible(v.hwnd) {
		win.SendMessage(v.hwnd, win.WM_SETREDRAW, 0, 0)
		defer func() { win.SendMessage(v.hwnd, win.WM_SETREDRAW, 1, 0); win.InvalidateRect(v.hwnd, nil, false) }()
	}
	change := logformat.Delta(v.text, text)
	if changed {
		change = logformat.Change{Add: text, Reset: true}
	}
	if change.Reset {
		selectLogRange(v.hwnd, 0, -1)
		replaceLogText(v.hwnd, "")
	} else if change.Drop > 0 {
		selectLogRange(v.hwnd, 0, logformat.Units(v.text[:change.Drop]))
		replaceLogText(v.hwnd, "")
	}
	if change.Add != "" {
		selectLogRange(v.hwnd, -1, -1)
		replaceLogText(v.hwnd, change.Add)
		start := len(text) - len(change.Add)
		start = strings.LastIndex(text[:start], "\n") + 1
		base := logformat.Units(text[:start])
		for _, r := range logformat.Runs(text[start:]) {
			selectLogRange(v.hwnd, base+r.Start, base+r.End)
			cf := win.CHARFORMAT{CbSize: uint32(unsafe.Sizeof(win.CHARFORMAT{})), DwMask: win.CFM_COLOR, CrTextColor: logColors[r.Color]}
			win.SendMessage(v.hwnd, win.EM_SETCHARFORMAT, win.SCF_SELECTION, uintptr(unsafe.Pointer(&cf)))
		}
	}
	win.SendMessage(v.hwnd, win.EM_EMPTYUNDOBUFFER, 0, 0)
	selectLogRange(v.hwnd, -1, -1)
	v.bottom()
	var current win.POINT
	win.SendMessage(v.hwnd, win.EM_GETSCROLLPOS, 0, uintptr(unsafe.Pointer(&current)))
	if !changed {
		current.X = pos.X
		win.SendMessage(v.hwnd, win.EM_SETSCROLLPOS, 0, uintptr(unsafe.Pointer(&current)))
	}
	v.actions = actions
	v.text = text
	v.source = source
	v.force = false
	v.layoutActions()
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
	if m.logs == nil || !win.IsWindowVisible(m.logs.hwnd) || win.IsIconic(m.window.Handle()) {
		return
	}
	names := []string{"console.log", "wallet-console.log", "mining-console.log", "tor-console.log", "kaspa-console.log", "dual-console.log", "kaspa-tor", "bot.log"}
	i := m.logSelect.CurrentIndex()
	if i < 0 || i >= len(names) {
		return
	}
	if i == 1 {
		m.refreshLogOwners()
	}
	path := filepath.Join(m.root, "logs", names[i])
	if i == 6 {
		path = filepath.Join(kaspaAccessRoot(m.root), "logs", "tor-console.log")
	}
	if path == m.logs.source && !m.logs.force && (m.logPause.Checked() || !m.logs.atBottom() || m.logs.selected()) {
		return
	}
	f, e := openSharedLog(path)
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
	if path == m.logs.source && !m.logs.force && m.logs.fileInfo != nil && os.SameFile(st, m.logs.fileInfo) && st.Size() == m.logs.fileInfo.Size() && st.ModTime().Equal(m.logs.fileInfo.ModTime()) {
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
	m.logs.fileInfo = st
}

var logColors = [...]win.COLORREF{
	0, 226 | 232<<8 | 240<<16, 148 | 163<<8 | 184<<16, 255 | 125<<8 | 125<<16,
	255 | 203<<8 | 107<<16, 103 | 216<<8 | 239<<16, 172 | 221<<8 | 188<<16,
}

func selectLogRange(h win.HWND, start, end int32) {
	r := win.CHARRANGE{CpMin: start, CpMax: end}
	win.SendMessage(h, win.EM_EXSETSEL, 0, uintptr(unsafe.Pointer(&r)))
}
func replaceLogText(h win.HWND, text string) {
	text = strings.ReplaceAll(text, "\x00", "�")
	b := windows.StringToUTF16(text)
	win.SendMessage(h, win.EM_REPLACESEL, 0, uintptr(unsafe.Pointer(&b[0])))
	runtime.KeepAlive(b)
}

// Allow the service to rotate logs while this window reads a bounded tail.
func openSharedLog(path string) (*os.File, error) {
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return nil, e
	}
	h, e := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(h), path), nil
}

func decorateCheckpointLog(text string) (string, []logAction) {
	var b strings.Builder
	var actions []logAction
	var units int32
	for _, line := range strings.SplitAfter(text, "\n") {
		b.WriteString(line)
		units += logformat.Units(line)
		if cursor := node.FailureCursor(line); cursor != "" {
			if !strings.HasSuffix(line, "\n") {
				b.WriteByte('\n')
				units++
			}
			b.WriteString("  Checkpoint cannot verify. ")
			units += logformat.Units("  Checkpoint cannot verify. ")
			actions = append(actions, logAction{position: units, cursor: cursor})
			b.WriteString("[No unique match]                       \n\n")
			units += logformat.Units("[No unique match]                       \n\n")
		}
	}
	return b.String(), actions
}
func (v *logView) disableCursor(cursor string) { delete(v.owners, cursor); v.layoutActions() }
func (v *logView) layoutActions() {
	if v.parent == nil {
		return
	}
	bounds := v.parent.ClientBoundsPixels()
	used := 0
	scale := v.parent.DPI()
	height, width := 24*scale/96, 190*scale/96
	for _, a := range v.actions {
		if len(v.owners[a.cursor]) != 1 {
			continue
		}
		var pos win.POINT
		win.SendMessage(v.hwnd, 0x0426, uintptr(unsafe.Pointer(&pos)), uintptr(a.position))
		if pos.Y < 0 || int(pos.Y)+height > bounds.Height || pos.X < 0 || int(pos.X)+width > bounds.Width {
			continue
		}
		if used >= 64 {
			break
		}
		if used >= len(v.buttons) {
			class, _ := windows.UTF16PtrFromString("BUTTON")
			label, _ := windows.UTF16PtrFromString("Rescan this wallet…")
			h := win.CreateWindowEx(0, class, label, win.WS_CHILD|win.WS_TABSTOP|win.BS_PUSHBUTTON, 0, 0, 1, 1, v.hwnd, win.HMENU(1000+len(v.buttons)), win.GetModuleHandle(nil), nil)
			if h == 0 {
				break
			}
			v.buttons = append(v.buttons, logButton{button: h})
		}
		item := &v.buttons[used]
		item.cursor = a.cursor
		item.token = v.owners[a.cursor][0]
		win.MoveWindow(item.button, pos.X, pos.Y, int32(width), int32(height), true)
		win.ShowWindow(item.button, win.SW_SHOWNA)
		win.SetWindowPos(item.button, win.HWND_TOP, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)
		used++
	}
	for i := used; i < len(v.buttons); i++ {
		v.buttons[i].cursor = ""
		win.ShowWindow(v.buttons[i].button, win.SW_HIDE)
	}
}
