package node

import (
	"fmt"
	"strconv"
	"strings"
)

// Zero values preserve the daemon's defaults and existing installations.
type WalletOptions struct {
	CompactNullifiers         bool `json:"compactNullifiers,omitempty"`
	SharedLeafStore           bool `json:"sharedLeafStore,omitempty"`
	HardMaxResident           int  `json:"hardMaxResident,omitempty"`
	ResidentBudgetMB          int  `json:"residentBudgetMB,omitempty"`
	SyncWallets               int  `json:"syncWallets,omitempty"`
	LoadWallets               int  `json:"loadWallets,omitempty"`
	ProofThreads              int  `json:"proofThreads,omitempty"`
	ConcurrentProves          int  `json:"concurrentProves,omitempty"`
	IdleEvict                 int  `json:"idleEvictSeconds,omitempty"`
	CheckpointInterval        int  `json:"checkpointIntervalSeconds,omitempty"`
	CatchupCheckpointInterval int  `json:"catchupCheckpointIntervalSeconds,omitempty"`
	PrefetchDepth             int  `json:"prefetchDepth,omitempty"`
}
type walletOption struct {
	flag       string
	value, max int
}

func (o WalletOptions) numbers() []walletOption {
	return []walletOption{
		{"hard-max-resident-wallets", o.HardMaxResident, 10000},
		{"resident-budget-mb", o.ResidentBudgetMB, 1048576},
		{"sync-wallets", o.SyncWallets, 1024},
		{"load-wallets", o.LoadWallets, 1024},
		{"proof-threads", o.ProofThreads, 4096},
		{"max-concurrent-proves", o.ConcurrentProves, 1024},
		{"idle-evict", o.IdleEvict, 86400},
		{"checkpoint-interval", o.CheckpointInterval, 86400},
		{"catchup-checkpoint-interval", o.CatchupCheckpointInterval, 86400},
		{"prefetch-depth", o.PrefetchDepth, 64},
	}
}
func (o WalletOptions) Validate() error {
	for _, n := range o.numbers() {
		if n.value < 0 || n.value > n.max {
			return fmt.Errorf("Wallet option --%s must be between 0 (daemon default) and %d", n.flag, n.max)
		}
	}
	return nil
}
func (o WalletOptions) Args() []string {
	var args []string
	if o.CompactNullifiers {
		args = append(args, "--compact-nullifiers")
	}
	if o.SharedLeafStore {
		args = append(args, "--shared-leaf-store")
	}
	for _, n := range o.numbers() {
		if n.value > 0 {
			args = append(args, "--"+n.flag+"="+strconv.Itoa(n.value))
		}
	}
	return args
}

// Check the installed binary's CLI rather than guessing from release numbers.
func (o WalletOptions) CheckHelp(help string) error {
	flags := map[string]bool{}
	for _, token := range strings.Fields(help) {
		flags[strings.TrimRight(token, ",")] = true
	}
	for _, arg := range o.Args() {
		flag := strings.SplitN(arg, "=", 2)[0]
		if !flags[flag] {
			return fmt.Errorf("Installed wallet daemon does not advertise %s. Update the ZKas node/wallet backend or disable that option in Settings → ZKas", flag)
		}
	}
	return nil
}
