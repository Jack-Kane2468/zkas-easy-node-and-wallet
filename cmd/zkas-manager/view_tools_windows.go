package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"zkas-node-manager/internal/node"
	"zkas-node-manager/internal/wallet"
)

type viewToolsUI struct {
	toolsKey                           *walk.LineEdit
	toolsMode                          *walk.ComboBox
	toolsSummary                       *walk.TextLabel
	toolsAddresses                     *walk.TextEdit
	toolsTable                         *walk.TableView
	toolsModel                         *viewHistoryModel
	toolsStart, toolsStop, toolsDerive *walk.PushButton
	toolsCancel                        context.CancelFunc
	toolsBusy                          bool
	toolsToken                         string
	toolsPort                          int
	ovkCursor, ovkKey                  string
	ovkBlocks, ovkMissing              int
	ovkRows                            []wallet.HistoryRow
}
type viewHistoryModel struct {
	walk.TableModelBase
	rows []wallet.HistoryRow
}

func (v *viewHistoryModel) RowCount() int { return len(v.rows) }
func (v *viewHistoryModel) Value(row, col int) interface{} {
	r := v.rows[row]
	switch col {
	case 0:
		return r.Kind
	case 1:
		return time.UnixMilli(int64(r.Timestamp)).Local().Format("2006-01-02 15:04")
	case 2:
		n, e := strconv.ParseUint(r.Amount, 10, 64)
		if e == nil {
			return wallet.FormatAmount(n)
		}
		return r.Amount
	case 3:
		return r.Recipient
	case 4:
		return r.TxID
	case 5:
		return r.Memo
	case 6:
		if r.FeeKnown {
			n, _ := strconv.ParseUint(r.Fee, 10, 64)
			return wallet.FormatAmount(n)
		}
		return "Unknown"
	}
	return ""
}
func (m *manager) walletToolsTab() TabPage {
	m.toolsModel = &viewHistoryModel{}
	return TabPage{Title: "Wallet tools", Layout: VBox{MarginsZero: true}, Children: []Widget{ScrollView{Layout: VBox{Spacing: 12}, Children: []Widget{
		sharingPanel("GENERAL VIEWING TOOLS", "Scan any viewing key", "No wallet login required. FVK scans incoming funds and available outgoing history. OVK scans recoverable outgoing outputs; it cannot show the sender's balance or identify which outputs are change.", walk.RGB(108, 54, 150), walk.RGB(246, 239, 252), []Widget{
			ComboBox{AssignTo: &m.toolsMode, Model: []string{"FVK — full viewing scan", "OVK — outgoing output scan"}, CurrentIndex: 0},
			Label{Text: "Viewing key (hex)"}, LineEdit{AssignTo: &m.toolsKey, PasswordMode: true},
			PushButton{AssignTo: &m.toolsStart, Text: "Start scan", OnClicked: func() { m.startViewScan(false) }},
			PushButton{Text: "Continue paused OVK scan", OnClicked: func() { m.startViewScan(true) }},
			PushButton{AssignTo: &m.toolsStop, Text: "Stop / pause this tool", Enabled: false, OnClicked: func() {
				if m.toolsCancel != nil {
					m.toolsCancel()
				}
			}},
			TextLabel{Text: "FVK scans register a separate view-only profile in the local wallet service, which retains the FVK and scan cache. OVKs stay in this window's memory. No spending key is needed. Full recovery requires the node to retain the needed history; private sends may not be recoverable.", MinSize: Size{Width: 240}},
		}),
		TextLabel{Text: "Results below belong to the last started scan. Editing the key does not change the displayed results until you start again.", MinSize: Size{Width: 240}},
		TextLabel{AssignTo: &m.toolsSummary, Text: "Paste an FVK or OVK to begin.", Font: Font{PointSize: 11, Bold: true}, MinSize: Size{Width: 240}},
		GroupBox{Title: "Receiving / recovered recipient addresses", Layout: VBox{}, Children: []Widget{
			TextLabel{Text: "FVK: receiving addresses discovered in the loaded history. OVK: recovered output recipients, which can include change. Unused receiving addresses cannot be discovered by scanning; derive an explicit range with an FVK instead.", MinSize: Size{Width: 240}},
			TextEdit{AssignTo: &m.toolsAddresses, ReadOnly: true, VScroll: true, HScroll: true, MinSize: Size{Height: 140}},
			PushButton{AssignTo: &m.toolsDerive, Text: "Derive FVK receiving addresses by index…", OnClicked: m.deriveToolAddresses},
			PushButton{Text: "Rebuild this FVK scan from genesis…", OnClicked: m.rescanToolFVK},
			PushButton{Text: "Copy displayed addresses", OnClicked: func() { m.copyText("Viewing-tool addresses", m.toolsAddresses.Text()) }},
		}},
		GroupBox{Title: "Recovered history / outputs", Layout: VBox{}, Children: []Widget{
			TableView{AssignTo: &m.toolsTable, Model: m.toolsModel, MinSize: Size{Height: 260}, Columns: []TableViewColumn{{Title: "Kind", Width: 110}, {Title: "Time (local)", Width: 145}, {Title: "Amount ZKAS", Width: 130}, {Title: "Address", Width: 260}, {Title: "Transaction", Width: 230}, {Title: "Memo", Width: 220}, {Title: "Fee ZKAS", Width: 100}}},
			PushButton{Text: "Copy selected row", OnClicked: func() {
				i := m.toolsTable.CurrentIndex()
				if i >= 0 && i < len(m.toolsModel.rows) {
					b, _ := json.MarshalIndent(m.toolsModel.rows[i], "", "  ")
					m.copyText("Recovered viewing data", string(b))
				}
			}},
		}},
	}}}}
}
func (m *manager) setToolsBusy(b bool) {
	m.toolsBusy = b
	m.toolsStart.SetEnabled(!b)
	m.toolsStop.SetEnabled(b)
	m.toolsKey.SetEnabled(!b)
	m.toolsMode.SetEnabled(!b)
	m.toolsDerive.SetEnabled(!b && m.toolsToken != "")
}
func (m *manager) showToolRows(summary string, rows []wallet.HistoryRow, ovk bool) {
	m.toolsSummary.SetText(summary)
	m.toolsModel.rows = rows
	m.toolsModel.PublishRowsReset()
	addresses := map[string]int{}
	for _, r := range rows {
		if r.Recipient != "" && (ovk || r.Kind == "received" || r.Kind == "coinbase") {
			addresses[r.Recipient]++
		}
	}
	names := []string{}
	for a := range addresses {
		names = append(names, a)
	}
	sort.Strings(names)
	lines := []string{}
	for _, a := range names {
		lines = append(lines, fmt.Sprintf("%s  (%d recorded outputs / rows)", a, addresses[a]))
	}
	if len(lines) == 0 {
		lines = append(lines, "No addresses discovered in the history loaded so far. This does not prove the wallet has no activity.")
	}
	m.toolsAddresses.SetText(strings.Join(lines, "\r\n"))
}
func (m *manager) startViewScan(resume bool) {
	if m.toolsBusy || m.busy {
		return
	}
	ovk := m.toolsMode.CurrentIndex() == 1
	size := 96
	if ovk {
		size = 32
	}
	key, e := wallet.CleanHex(m.toolsKey.Text(), size)
	if e != nil {
		m.walletError(e)
		return
	}
	if resume && (!ovk || m.ovkCursor == "" || key != m.ovkKey) {
		m.walletError(fmt.Errorf("choose the same OVK as the paused scan, or start a new scan"))
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.toolsCancel = cancel
	m.setToolsBusy(true)
	m.toolsToken = ""
	m.toolsSummary.SetText("Starting viewing scan…")
	c := m.cfg
	if ovk {
		if !resume {
			m.ovkCursor = node.MainnetGenesis
			m.ovkKey = key
			m.ovkRows = nil
			m.ovkBlocks = 0
			m.ovkMissing = 0
		}
		cursor, prior, blocks, missing := m.ovkCursor, append([]wallet.HistoryRow(nil), m.ovkRows...), m.ovkBlocks, m.ovkMissing
		go func() {
			e := m.scanOVK(ctx, c.GRPC, key, cursor, prior, blocks, missing)
			m.onUI(func() {
				m.setToolsBusy(false)
				if e != nil && ctx.Err() == nil {
					m.toolsSummary.SetText("OVK scan incomplete: " + e.Error())
				}
			})
			cancel()
		}()
	} else {
		go func() {
			e := m.scanFVK(ctx, c.WalletPort, key)
			m.onUI(func() {
				m.setToolsBusy(false)
				if e != nil && ctx.Err() == nil {
					m.toolsSummary.SetText("FVK scan incomplete: " + e.Error())
				}
			})
			cancel()
		}()
	}
}
func (m *manager) scanFVK(ctx context.Context, port int, key string) error {
	token := wallet.ViewingToken(key)
	client := wallet.Client{Port: port, Token: token}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	// Revalidate the supplied FVK against the dedicated tool profile. Same-key
	// registration preserves its checkpoint in the upstream daemon.
	if e := client.Watch(cctx, key); e != nil {
		return e
	}
	cancel()
	m.onUI(func() { m.toolsToken = token; m.toolsPort = port })
	for {
		s, rows, total, e := readViewingSnapshot(ctx, client)
		if e != nil {
			return e
		}
		if s.Loading {
			m.onUI(func() { m.toolsSummary.SetText("Opening the view-only wallet. Waiting for its scan snapshot…") })
		} else {
			summary := fmt.Sprintf("%s\nScanned %d / %d blocks · loaded %d of %d history rows", s.Display(), s.Scanned, s.Chain, len(rows), total)
			if len(rows) < total {
				summary += "\nHistory display capped at 100,000 rows; address list may be incomplete."
			}
			if s.MissingHistory {
				summary += "\nINCOMPLETE: this node cannot serve all required history. Balances and address history may be partial."
			}
			m.onUI(func() {
				m.showToolRows(summary, rows, false)
				m.toolsAddresses.SetText("Default receiving address: " + s.Address + "\r\n\r\n" + m.toolsAddresses.Text())
			})
			if s.Synced || s.MissingHistory {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			m.onUI(func() {
				m.toolsSummary.SetText("Tool paused. The local wallet daemon may continue scanning this view-only profile.")
			})
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}

	}
}
func (m *manager) deriveToolAddresses() {
	if m.toolsBusy || m.toolsToken == "" {
		m.walletError(fmt.Errorf("scan an FVK first"))
		return
	}
	values, ok := m.inputDialog("FVK receiving addresses", "Derive a specific range. These are valid addresses for this FVK, not evidence that they have been used.", []inputField{{"First index", "0", false}, {"Number of addresses (1–100)", "20", false}})
	if !ok {
		return
	}
	start, e := strconv.ParseUint(values[0], 10, 32)
	count, ce := strconv.Atoi(values[1])
	if e != nil || ce != nil || count < 1 || count > 100 || start+uint64(count) > 1<<32 {
		m.walletError(fmt.Errorf("enter a valid index range and count 1–100"))
		return
	}
	client := wallet.Client{Port: m.toolsPort, Token: m.toolsToken}
	m.action(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		lines := []string{"Derived addresses — usage not checked:"}
		for i := uint64(0); i < uint64(count); i++ {
			var a struct {
				Address string `json:"address"`
			}
			if e := client.Do(ctx, "GET", fmt.Sprintf("/api/wallet/address?index=%d", start+i), nil, &a); e != nil {
				return e
			}
			lines = append(lines, fmt.Sprintf("%d  %s", start+i, a.Address))
		}
		m.onUI(func() { m.toolsAddresses.SetText(strings.Join(lines, "\r\n")) })
		return nil
	})
}
func (m *manager) scanOVK(ctx context.Context, port int, key, cursor string, rows []wallet.HistoryRow, blocks, missing int) error {
	rpc, e := node.NewViewRPC(port)
	if e != nil {
		return e
	}
	defer rpc.Close()
	if cursor == node.MainnetGenesis {
		if e = rpc.CheckGenesis(ctx); e != nil {
			return e
		}
	}
	dir, e := walletRuntime(m.root)
	if e != nil {
		return e
	}
	cmd := exec.CommandContext(ctx, filepath.Join(dir, "node.exe"), "--no-warnings", filepath.Join(dir, "view-tools.mjs"))
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	for _, v := range os.Environ() {
		u := strings.ToUpper(v)
		if !strings.HasPrefix(u, "NODE_") && !strings.HasPrefix(u, "ELECTRON_") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	in, e := cmd.StdinPipe()
	if e != nil {
		return e
	}
	out, e := cmd.StdoutPipe()
	if e != nil {
		return e
	}
	if e = cmd.Start(); e != nil {
		return e
	}
	defer func() { in.Close(); cmd.Process.Kill(); cmd.Wait() }()
	reader := bufio.NewScanner(out)
	reader.Buffer(make([]byte, 65536), 2<<20)
	type recovered struct {
		Action int    `json:"action"`
		Raw    string `json:"recipient_raw"`
		Amount string `json:"amount_sompi"`
		Memo   string `json:"memo"`
	}
	update := func(note string) {
		copyRows := append([]wallet.HistoryRow(nil), rows...)
		savedCursor, savedBlocks, savedMissing := cursor, blocks, missing
		m.onUI(func() {
			m.ovkRows = copyRows
			m.ovkCursor = savedCursor
			m.ovkBlocks = savedBlocks
			m.ovkMissing = savedMissing
			m.showToolRows(fmt.Sprintf("%s\n%d chain blocks scanned · %d recovered outputs · %d accepted transaction bodies unavailable\nRecovered outputs can include change; totals are not net payments or a balance.", note, savedBlocks, len(copyRows), savedMissing), copyRows, true)
		})
	}
	for {
		page, e := rpc.Page(ctx, cursor)
		if e != nil {
			update("Stopped — results are partial; restart if a reorganization was reported.")
			return e
		}
		if len(page) == 0 {
			update("Reached the node's current scan tip. Results are incomplete if any transaction bodies were unavailable.")
			return nil
		}
		for _, b := range page {
			if b.Hash == cursor {
				return fmt.Errorf("node returned a non-advancing scan cursor")
			}
			bundles, lost, e := rpc.Bundles(ctx, b)
			if e != nil {
				update("Paused — resume to continue.")
				return e
			}
			pending := []wallet.HistoryRow{}
			for tx, payload := range bundles {
				req, _ := json.Marshal(map[string]string{"ovk": key, "payload": payload})
				if _, e = in.Write(append(req, '\n')); e != nil {
					return fmt.Errorf("OVK recovery helper stopped")
				}
				clear(req)
				if !reader.Scan() {
					return fmt.Errorf("OVK recovery helper stopped")
				}
				var reply struct {
					OK   bool        `json:"ok"`
					Rows []recovered `json:"rows"`
				}
				if json.Unmarshal(reader.Bytes(), &reply) != nil || !reply.OK {
					return fmt.Errorf("node supplied an unsupported shielded bundle; scan is incomplete")
				}
				for _, r := range reply.Rows {
					raw, e := hex.DecodeString(r.Raw)
					if e != nil {
						return e
					}
					address, e := wallet.EncodeReceiver(raw)
					if e != nil {
						return e
					}
					pending = append(pending, wallet.HistoryRow{Kind: fmt.Sprintf("output %d", r.Action), TxID: tx, Timestamp: b.Timestamp, DAA: b.DAA, Amount: r.Amount, Recipient: address, Memo: r.Memo})
				}
			}
			rows = append(rows, pending...)
			missing += lost
			blocks++
			cursor = b.Hash
			if len(rows) >= 100000 {
				update("Paused at the 100,000-output display limit.")
				return nil
			}
			if ctx.Err() != nil {
				update("Paused — resume to continue.")
				return ctx.Err()
			}
		}
		update("Scanning OVK against accepted on-chain transactions…")
	}
}

func readViewingSnapshot(ctx context.Context, client wallet.Client) (wallet.Status, []wallet.HistoryRow, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	s, e := client.Status(ctx)
	if e != nil {
		return s, nil, 0, e
	}
	if s.Loading {
		return s, nil, 0, nil
	}
	if !s.HasWallet {
		return s, nil, 0, fmt.Errorf("view-only wallet is not available in the daemon")
	}
	rows := []wallet.HistoryRow{}
	total := 0
	for offset := 0; offset < 100000; {
		p, e := client.HistoryPage(ctx, offset)
		if e != nil {
			return s, rows, total, e
		}
		total = p.Total
		rows = append(rows, p.Rows...)
		offset += len(p.Rows)
		if offset >= total || len(p.Rows) == 0 {
			break
		}
	}
	return s, rows, total, nil
}

func (m *manager) rescanToolFVK() {
	if m.toolsBusy || m.toolsToken == "" || !strings.HasPrefix(m.toolsToken, "viewtool-") {
		m.walletError(fmt.Errorf("finish or pause an FVK scan first"))
		return
	}
	if walk.MsgBox(m.window, "Rebuild viewing scan", "Rebuild only this tool's view-only profile from genesis? This may take time and temporarily clears its scan cache. It does not change your signed-in wallet profiles or spend funds.", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	client := wallet.Client{Port: m.toolsPort, Token: m.toolsToken}
	m.action(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		e := client.Do(ctx, "POST", "/api/wallet/rescan", map[string]any{"birthday": 0}, nil)
		if e == nil {
			m.onUI(func() { m.toolsSummary.SetText("View-only rescan requested. Click Start scan to follow progress.") })
		}
		return e
	})
}
