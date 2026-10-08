package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"zkas-node-manager/internal/node"
)

func miningHostPath(root string) (string, error) {
	source, e := os.Executable()
	if e != nil {
		return "", e
	}
	hash, e := node.FileHash(source)
	if e != nil {
		return "", e
	}
	dest := filepath.Join(root, "mining", "ManagerHost-"+node.ManagerVersion+"-"+hash[:12]+".exe")
	if h, _ := node.FileHash(dest); h == hash {
		return dest, nil
	}
	if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
		return "", e
	}
	b, e := os.ReadFile(source)
	if e != nil {
		return "", e
	}
	return dest, os.WriteFile(dest, b, 0700)
}

// This deliberately installs into mining/versions, never the active node folder.
func installMining(root string, c node.Config, mc node.MiningConfig, progress func(string)) (node.MiningConfig, error) {
	if e := mc.Validate(); e != nil {
		return mc, e
	}
	if serviceActive(root, "mining") {
		return mc, fmt.Errorf("Stop mining before changing or updating its bridge")
	}
	if mc.Version != c.Version || node.CheckMiningBinary(mc) != nil {
		progress("Downloading the mining bridge matching node " + c.Version + "…")
		r, e := node.ReleaseAt(context.Background(), c.Version)
		if e != nil {
			return mc, e
		}
		nodeExe, _, e := node.Install(context.Background(), filepath.Join(root, "mining"), r, false, progress, "stratum-bridge.exe")
		if e != nil {
			return mc, e
		}
		mc.Executable = filepath.Join(filepath.Dir(nodeExe), "stratum-bridge.exe")
		mc.SHA256, e = node.FileHash(mc.Executable)
		if e != nil {
			return mc, e
		}
		mc.Version = r.Tag
		// Only the bridge is needed here. The running node binary is elsewhere.
		if e = os.Remove(nodeExe); e != nil {
			return mc, e
		}
	}
	host, e := miningHostPath(root)
	if e != nil {
		return mc, e
	}
	mc.HostExecutable = host
	// Explicit minimal config prevents discovery of an unrelated config.yaml.
	if e = os.WriteFile(filepath.Join(root, "mining", "bridge-config.yaml"), []byte("{}\n"), 0600); e != nil {
		return mc, e
	}
	return mc, node.SaveMining(root, mc)
}
func runPS(script string) error {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, e := cmd.CombinedOutput()
	if e != nil {
		return fmt.Errorf("%w: %s", e, strings.TrimSpace(string(output)))
	}
	return nil
}

const firewallRule = "ZKasNodeManager-Stratum-5555"

func firewallReady(mc node.MiningConfig) bool {
	script := "$ErrorActionPreference='Stop';$r=Get-NetFirewallRule -Name " + psQuote(firewallRule) + " -ErrorAction SilentlyContinue; if(-not $r -or $r.Enabled -ne 'True' -or $r.Direction -ne 'Inbound' -or $r.Action -ne 'Allow' -or [string]$r.Profile -ne 'Private'){exit 1};$p=$r|Get-NetFirewallPortFilter;$a=$r|Get-NetFirewallAddressFilter;$app=$r|Get-NetFirewallApplicationFilter;if($p.LocalPort -ne '5555' -or $p.Protocol -ne 'TCP' -or [string]$a.RemoteAddress -ne 'LocalSubnet' -or $app.Program -ne " + psQuote(mc.Executable) + "){exit 1}"
	return runPS(script) == nil
}
func allowMiningLAN(mc node.MiningConfig) error {
	if firewallReady(mc) {
		return nil
	}
	// Windows displays its normal UAC approval. No node/wallet port is opened.
	return runPS("$ErrorActionPreference='Stop';$p=Start-Process -FilePath " + psQuote(mc.HostExecutable) + " -ArgumentList '--allow-mining-lan' -Verb RunAs -Wait -PassThru;if($p.ExitCode -ne 0){throw 'Unable to create the mining firewall rule. Check mining-firewall-error.txt.'}")
}
func firewallHelper() error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	root := filepath.Dir(filepath.Dir(exe))
	mc, e := node.ReadMining(root)
	if e != nil {
		return e
	}
	if e = node.CheckMiningBinary(mc); e != nil {
		return e
	}
	script := "$ErrorActionPreference='Stop';Get-NetFirewallRule -Name " + psQuote(firewallRule) + " -ErrorAction SilentlyContinue | Remove-NetFirewallRule; New-NetFirewallRule -Name " + psQuote(firewallRule) + " -DisplayName 'ZKas mining - TCP 5555 (private LAN)' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 5555 -Profile Private -RemoteAddress LocalSubnet -Program " + psQuote(mc.Executable) + " | Out-Null"
	e = runPS(script)
	if e != nil {
		os.WriteFile(filepath.Join(root, "mining-firewall-error.txt"), []byte(e.Error()), 0600)
	}
	return e
}
func startMining(root string, c node.Config, mc node.MiningConfig) error {
	if serviceActive(root, "dual") {
		return fmt.Errorf("Stop shared mining first; mining modes share port 5555")
	}
	if serviceActive(root, "mining") {
		return fmt.Errorf("Mining bridge is already running")
	}
	if !serviceActive(root, "node") {
		return fmt.Errorf("Start the node first")
	}
	info, e := node.GetInfo(context.Background(), c.GRPC)
	if e != nil {
		return fmt.Errorf("Node RPC is not ready: %w", e)
	}
	if !info.Synced {
		return fmt.Errorf("Wait for consensus sync to finish before starting mining")
	}
	if mc.Version != c.Version {
		return fmt.Errorf("Install the mining bridge that matches the current node release")
	}
	if e = node.CheckMiningBinary(mc); e != nil {
		return e
	}
	return startOne(root, c, "mining")
}

func closeMiningLAN(mc node.MiningConfig) error {
	if mc.HostExecutable == "" {
		return nil
	}
	return runPS("$ErrorActionPreference='Stop';$p=Start-Process -FilePath " + psQuote(mc.HostExecutable) + " -ArgumentList '--remove-mining-lan' -Verb RunAs -Wait -PassThru;if($p.ExitCode -ne 0){throw 'Unable to remove the mining firewall rule.'}")
}
func removeMiningFirewallHelper() error {
	return runPS("$ErrorActionPreference='Stop';Get-NetFirewallRule -Name " + psQuote(firewallRule) + " -ErrorAction SilentlyContinue | Remove-NetFirewallRule")
}

// Reused by bridge-only and whole-manager uninstall. Keep payout preferences.
func uninstallMining(root string) error {
	if e := stopOne(root, "mining"); e != nil {
		return e
	}
	mc, e := node.ReadMining(root)
	if e != nil {
		return e
	}
	if mc.HostExecutable != "" || mc.Executable != "" {
		// Use this manager, even if the old bridge host is missing or broken.
		mc.HostExecutable, e = os.Executable()
		if e != nil {
			return e
		}
		if e = closeMiningLAN(mc); e != nil {
			return e
		}
	}
	if e = os.RemoveAll(filepath.Join(root, "mining")); e != nil {
		return e
	}
	mc.Executable, mc.SHA256, mc.Version, mc.HostExecutable = "", "", "", ""
	return node.SaveMining(root, mc)
}
