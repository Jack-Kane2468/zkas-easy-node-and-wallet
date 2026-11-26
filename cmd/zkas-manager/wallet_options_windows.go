package main

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"zkas-node-manager/internal/node"
)

type walletOptionsUI struct {
	compactNullifiers, sharedLeafStore                                                     *walk.CheckBox
	daemonAdvanced                                                                         *walk.Composite
	hardResident, residentBudget, syncWallets, loadWallets, proofThreads, concurrentProves *walk.NumberEdit
	idleEvict, checkpointInterval, catchupInterval, prefetchDepth                          *walk.NumberEdit
	walletOptionsStatus                                                                    *walk.TextLabel
	walletOptionsSave, walletOptionsCheck                                                  *walk.PushButton
}

func (m *manager) walletDaemonSettings() Widget {
	o := m.cfg.WalletDaemon
	advancedOptions := o
	advancedOptions.CompactNullifiers, advancedOptions.SharedLeafStore = false, false
	advanced := len(advancedOptions.Args()) > 0
	var showAdvanced *walk.CheckBox
	row := func(label, help string, p **walk.NumberEdit, value, max int) Widget {
		return Composite{Layout: VBox{MarginsZero: true, Spacing: 3}, Children: []Widget{
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{Label{Text: label}, NumberEdit{AssignTo: p, MinValue: 0, MaxValue: float64(max), Decimals: 0, Value: float64(value)}}},
			TextLabel{Text: help, MinSize: Size{Width: 240}},
		}}
	}
	return GroupBox{Title: "Wallet service · disk space and performance", Layout: VBox{Spacing: 6}, Children: []Widget{
		TextLabel{Text: "The wallet service scans the blockchain and saves each wallet’s balance and scan progress on this PC. These settings control its disk space, RAM and CPU use for all connected wallets, including bots and other apps. Stop ZKas services before editing; save and start them again to apply changes.", MinSize: Size{Width: 240}},
		CheckBox{AssignTo: &m.compactNullifiers, Text: "Reduce wallet disk space: compact spent-note records", Checked: o.CompactNullifiers},
		TextLabel{Text: "Stores the records used to identify spent notes (nullifiers) once, with a small index for each wallet. This reduces duplicated data in wallet scan files and the amount written when scan progress is saved. Most useful when this service handles many wallets. Uses --compact-nullifiers.", MinSize: Size{Width: 240}},
		CheckBox{AssignTo: &m.sharedLeafStore, Text: "Reduce wallet disk space: share transaction-tree data", Checked: o.SharedLeafStore},
		TextLabel{Text: "Lets wallets refer to one shared copy of transaction-tree data instead of storing a copy in every wallet’s scan file. Most useful for large scan files or many wallets on the same service. Uses --shared-leaf-store.", MinSize: Size{Width: 240}},
		TextLabel{Text: "Compatibility: shared transaction-tree data changes the saved scan format. Wallet daemons predating format v11 cannot read converted files; turning this option off does not undo conversion. Before enabling it, stop services and back up the complete wallet data folder. Keep the shared data files with the wallet scan files when backing up or moving them.", MinSize: Size{Width: 240}},
		TextLabel{Text: "What changes: wallet scan files can use less disk space as wallets load and the service saves their progress. The reduction is not immediate or guaranteed to be the same for every wallet. These options do not shrink the node’s blockchain folder or change wallet balances. Old recovery copies may still occupy disk space; Review wallet disk usage shows them without deleting anything.", MinSize: Size{Width: 240}},
		row("Wallets kept in RAM · target", "Keeps a target number of wallet checkpoints in memory so they can resume quickly. Recently active wallets can exceed this target. Lower values can reduce RAM use but cause more reloading. 0 lets the service choose.", &m.resident, m.cfg.ResidentWallets, 10000),
		CheckBox{AssignTo: &showAdvanced, Text: "Show advanced RAM, CPU and scan controls", Checked: advanced, OnCheckedChanged: func() {
			if m.daemonAdvanced != nil {
				m.daemonAdvanced.SetVisible(showAdvanced.Checked())
			}
		}},
		Composite{AssignTo: &m.daemonAdvanced, Visible: advanced, Layout: VBox{MarginsZero: true, Spacing: 6}, Children: []Widget{
			TextLabel{Text: "Leave these values at 0 unless you need to reduce resource use or tune a busy wallet service. Each control below explains what it limits and the tradeoff. Changing them does not change the amount of coins in a wallet.", MinSize: Size{Width: 240}},
			row("Wallets kept in RAM · maximum", "Limits how many wallet checkpoints can stay loaded. Opening another wallet may unload an idle one or wait if all are busy. Use this when many wallets cause excessive RAM use. 0 adds no hard wallet-count limit.", &m.hardResident, o.HardMaxResident, 10000),
			row("Wallet RAM budget (MiB)", "Limits the estimated memory used by loaded wallet checkpoints. This is not a limit on the whole service’s RAM: shared data and other work also use memory. Lower budgets can cause more reloading. 0 uses the service default; 1,024 MiB = 1 GiB.", &m.residentBudget, o.ResidentBudgetMB, 1048576),
			row("Wallets scanning at once", "Controls how many wallets can scan blockchain history at the same time. Lower values reduce competing RAM, CPU and disk work; higher values may help several wallets catch up together but can overload the PC. 0 uses the service default.", &m.syncWallets, o.SyncWallets, 1024),
			row("Wallet files loading at once", "Controls how many saved wallet scan files can be loaded at the same time. Lower this if opening many wallets causes a RAM or disk-activity spike; other wallets may wait longer to open. 0 uses the service default.", &m.loadWallets, o.LoadWallets, 1024),
			row("CPU threads for payment proofs", "Limits CPU threads available for building payment proofs. Lower this to leave CPU capacity for the node or other apps, at the cost of slower payment preparation. 0 uses the service default.", &m.proofThreads, o.ProofThreads, 4096),
			row("Payment proofs running at once", "Limits how many payment proofs can be prepared simultaneously across wallets. Lower values reduce peak CPU demand; additional requests may wait or need to retry. 0 uses the service default.", &m.concurrentProves, o.ConcurrentProves, 1024),
			row("Unload idle wallets after (seconds)", "Releases an unused wallet’s checkpoint from RAM after this period. Its disk files and funds remain intact, but reopening it may take longer. Lower values suit infrequently used wallets. 0 uses the service default, not immediate unloading.", &m.idleEvict, o.IdleEvict, 86400),
			row("Save scan progress every (seconds)", "Controls the time-based saving of wallet scan progress when the wallet has advanced. Longer intervals reduce disk writes but may require more re-scanning after a crash or forced shutdown. Important wallet state is saved separately. 0 uses the service default.", &m.checkpointInterval, o.CheckpointInterval, 86400),
			row("Save catching-up scan progress every (seconds)", "Overrides the save interval only for wallets far behind the chain. A longer interval can reduce heavy disk writes during a restore, but more progress may need to be scanned again after a crash. 0 follows the normal checkpoint interval.", &m.catchupInterval, o.CatchupCheckpointInterval, 86400),
			row("Blockchain pages fetched ahead", "Controls how many pages of blockchain history the service fetches concurrently ahead of a deep scan. Higher values may reduce waiting for data, but increase RAM use and load on the node’s disk. Leave at 0 for the service default unless tuning a slow restore.", &m.prefetchDepth, o.PrefetchDepth, 64),
			TextLabel{Text: "Lower proof-thread limits leave CPU for the node but make payments slower. Longer checkpoint intervals reduce writes but increase re-scanning after an unclean exit. RAM eviction does not delete wallet data.", MinSize: Size{Width: 240}},
		}},
		Composite{Layout: HBox{}, Children: []Widget{
			PushButton{AssignTo: &m.walletOptionsSave, Text: "Save settings", OnClicked: m.saveZKasSettings},
			PushButton{AssignTo: &m.walletOptionsCheck, Text: "Check installed service support", OnClicked: m.checkWalletOptions},
		}},
		Composite{Layout: HBox{}, Children: []Widget{
			PushButton{Text: "Review wallet disk usage…", OnClicked: m.reviewWalletStorage},
			PushButton{Text: "Open wallet data folder", OnClicked: func() { m.openWalletData() }},
		}},
		TextLabel{AssignTo: &m.walletOptionsStatus, Text: "Storage optimizations are opt-in. Existing settings are preserved.", MinSize: Size{Width: 240}},
	}}
}
func (m *manager) readWalletOptions() node.WalletOptions {
	return node.WalletOptions{CompactNullifiers: m.compactNullifiers.Checked(), SharedLeafStore: m.sharedLeafStore.Checked(), HardMaxResident: int(m.hardResident.Value()), ResidentBudgetMB: int(m.residentBudget.Value()), SyncWallets: int(m.syncWallets.Value()), LoadWallets: int(m.loadWallets.Value()), ProofThreads: int(m.proofThreads.Value()), ConcurrentProves: int(m.concurrentProves.Value()), IdleEvict: int(m.idleEvict.Value()), CheckpointInterval: int(m.checkpointInterval.Value()), CatchupCheckpointInterval: int(m.catchupInterval.Value()), PrefetchDepth: int(m.prefetchDepth.Value())}
}
func (m *manager) refreshWalletOptions(active bool) {
	if m.compactNullifiers == nil {
		return
	}
	for _, w := range []walk.Widget{m.compactNullifiers, m.sharedLeafStore, m.hardResident, m.residentBudget, m.syncWallets, m.loadWallets, m.proofThreads, m.concurrentProves, m.idleEvict, m.checkpointInterval, m.catchupInterval, m.prefetchDepth, m.walletOptionsSave, m.walletOptionsCheck} {
		w.SetEnabled(!m.busy && !active)
	}
	if active {
		m.walletOptionsStatus.SetText("Stop ZKas services in Overview → ZKas before changing these settings.")
	} else {
		m.walletOptionsStatus.SetText("Save settings, then start ZKas services in Overview to apply them.")
	}
}
func checkWalletOptionSupport(c node.Config) error {
	if !c.EnableWallet || len(c.WalletDaemon.Args()) == 0 {
		return nil
	}
	if e := node.CheckBinary(c); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.WalletExecutable, "--help")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, e := cmd.CombinedOutput()
	if e != nil {
		return fmt.Errorf("Cannot check installed wallet daemon options: %w", e)
	}
	return c.WalletDaemon.CheckHelp(string(out))
}
func (m *manager) checkWalletOptions() {
	if m.busy {
		return
	}
	c := m.cfg
	c.WalletDaemon = m.readWalletOptions()
	if e := c.WalletDaemon.Validate(); e != nil {
		m.walletError(e)
		return
	}
	if !c.EnableWallet || c.WalletExecutable == "" {
		m.walletError(fmt.Errorf("Enable and install the wallet REST backend first"))
		return
	}
	m.chainAction("Checking wallet daemon options", "Selected daemon options are supported. Save settings and start ZKas services to apply them.", func() error { return checkWalletOptionSupport(c) })
}
