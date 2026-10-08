package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/node"
)

func chainHostPath(root string) (string, error) { return copyManagerHost(root, "components") }
func copyManagerHost(root, folder string) (string, error) {
	src, e := os.Executable()
	if e != nil {
		return "", e
	}
	h, e := node.FileHash(src)
	if e != nil {
		return "", e
	}
	dst := filepath.Join(root, folder, "ManagerHost-"+node.ManagerVersion+"-"+h[:12]+".exe")
	if x, _ := node.FileHash(dst); x == h {
		return dst, nil
	}
	if e = os.MkdirAll(filepath.Dir(dst), 0700); e != nil {
		return "", e
	}
	b, e := os.ReadFile(src)
	if e != nil {
		return "", e
	}
	return dst, os.WriteFile(dst, b, 0700)
}
func chainPathGuard(root string) error {
	c, e := node.ReadConfig(root)
	if e != nil {
		return e
	}
	return validateData(root, c)
}
func installKaspa(root string, progress func(string)) error {
	if e := chainPathGuard(root); e != nil {
		return e
	}
	if serviceActive(root, "kaspa") || serviceActive(root, "dual") {
		return errors.New("Stop Kaspa and shared mining before installing Kaspa components")
	}
	c, e := chains.Read(root)
	if e != nil {
		return e
	}
	b, e := chains.Install(context.Background(), root, false, progress)
	if e != nil {
		return e
	}
	c.Kaspa, c.KaspaBridge, c.Wallet = b["kaspad.exe"], b["stratum-bridge.exe"], b["kaspa-wallet.exe"]
	c.Host, e = chainHostPath(root)
	if e != nil {
		return e
	}
	return chains.Save(root, c)
}
func installDual(root string, progress func(string)) error {
	if e := chainPathGuard(root); e != nil {
		return e
	}
	if serviceActive(root, "dual") {
		return errors.New("Stop shared mining before installing its bridge")
	}
	c, e := chains.Read(root)
	if e != nil {
		return e
	}
	b, e := chains.Install(context.Background(), root, true, progress)
	if e != nil {
		return e
	}
	c.DualBridge = b["stratum-bridge.exe"]
	c.Host, e = chainHostPath(root)
	if e != nil {
		return e
	}
	return chains.Save(root, c)
}
func checkKaspaIsolation(root string, c node.Config) error {
	a, b := strings.ToLower(filepath.Clean(chains.DataDir(root))), strings.ToLower(filepath.Clean(c.DataDir))
	sep := string(os.PathSeparator)
	if a == b || strings.HasPrefix(a, b+sep) || strings.HasPrefix(b, a+sep) {
		return errors.New("Choose separate ZKas and Kaspa data folders; their paths overlap")
	}
	kc, err := chains.Read(root)
	if err != nil {
		return err
	}
	if err = kc.ValidateNode(); err != nil {
		return err
	}
	for _, p := range []int{kc.GRPC, kc.P2P, kc.Borsh, kc.JSON} {
		if c.GRPC == p || c.EnableWebSocket && c.WebSocket == p || c.EnableWallet && c.WalletPort == p {
			return fmt.Errorf("Kaspa needs port %d, currently assigned to ZKas. Change the ZKas port in Settings first", p)
		}
	}
	return nil
}
func startKaspa(root string, c node.Config) error {
	if e := checkKaspaIsolation(root, c); e != nil {
		return e
	}
	s, e := chains.Read(root)
	if e != nil {
		return e
	}
	if e = s.Kaspa.Check(); e != nil {
		return e
	}
	if e = os.MkdirAll(chains.DataDir(root), 0700); e != nil {
		return e
	}
	s.Host, e = chainHostPath(root)
	if e != nil {
		return e
	}
	if e = chains.Save(root, s); e != nil {
		return e
	}
	if e = ensureKaspaPeerFirewall(root, s); e != nil {
		return e
	}
	return startOne(root, c, "kaspa")
}
func stopKaspa(root string) error {
	if e := stopOne(kaspaAccessRoot(root), "sharing"); e != nil {
		return e
	}
	c, e := chains.Read(root)
	if e != nil {
		return e
	}
	if c.Mode != "zkas" {
		if e := stopOne(root, "dual"); e != nil {
			return e
		}
	}
	return stopOne(root, "kaspa")
}
func startDual(root string, c node.Config, s chains.Config) error {
	if e := s.ValidateMining(); e != nil {
		return e
	}
	if serviceActive(root, "dual") || serviceActive(root, "mining") {
		return errors.New("Stop the running mining bridge first. All mining modes share TCP 5555")
	}
	if s.Mode != "zkas" && !serviceActive(root, "kaspa") {
		return errors.New("Install and start Kaspa in Overview → Kaspa first")
	}
	if e := s.Bridge().Check(); e != nil {
		return e
	}
	if s.Mode != "zkas" {
		if e := node.ValidatePayout(context.Background(), s.GRPC, s.KaspaAddress); e != nil {
			return fmt.Errorf("Kaspa payout / sync check: %w", e)
		}
	}
	if s.Mode != "kaspa" {
		if !serviceActive(root, "node") {
			return errors.New("Start ZKas in Overview first")
		}
		if e := node.ValidatePayout(context.Background(), c.GRPC, s.ZKasAddress); e != nil {
			return fmt.Errorf("ZKas payout / sync check: %w", e)
		}
	}
	old, e := chains.Read(root)
	if e != nil {
		return e
	}
	if old.LAN && !s.LAN {
		if e = chainFirewall(root, true); e != nil {
			return e
		}
	}
	b, e := s.BridgeConfig(c.GRPC)
	if e != nil {
		return e
	}
	s.Host, e = chainHostPath(root)
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(root, "components", "bridge.yaml"), b, 0600); e != nil {
		return e
	}
	if e = chains.Save(root, s); e != nil {
		return e
	}
	if s.LAN {
		if e = chainFirewall(root, false); e != nil {
			return e
		}
	}
	return startOne(root, c, "dual")
}

const chainFirewallRule = "ZKasNodeManager-Shared-Stratum-5555"

func chainFirewall(root string, remove bool) error {
	host, e := chainHostPath(root)
	if e != nil {
		return e
	}
	arg := "--allow-chain-lan"
	if remove {
		arg = "--remove-chain-lan"
	}
	return runPS("$ErrorActionPreference='Stop';$p=Start-Process -FilePath " + psQuote(host) + " -ArgumentList " + psQuote(arg) + " -Verb RunAs -Wait -PassThru;if($p.ExitCode -ne 0){throw 'Windows could not update the shared mining firewall rule.'}")
}
func chainFirewallHelper(root string, remove bool) error {
	script := "$ErrorActionPreference='Stop';Get-NetFirewallRule -Name '" + chainFirewallRule + "' -ErrorAction SilentlyContinue | Remove-NetFirewallRule"
	if !remove {
		c, e := chains.Read(root)
		if e != nil {
			return e
		}
		if !c.LAN {
			return errors.New("LAN mining is not enabled")
		}
		if e = c.Bridge().Check(); e != nil {
			return e
		}
		script += ";New-NetFirewallRule -Name '" + chainFirewallRule + "' -DisplayName 'Kaspa / ZKas shared mining (private LAN)' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 5555 -Profile Private -RemoteAddress LocalSubnet -Program " + psQuote(c.Bridge().Path) + " | Out-Null"
	}
	return runPS(script)
}
func uninstallChains(root string, bridgeOnly bool) error {
	if !bridgeOnly {
		if e := stopOne(kaspaAccessRoot(root), "sharing"); e != nil {
			return e
		}
		if e := removeKaspaAccessRules(root); e != nil {
			return e
		}
		if e := setKaspaAutoStart(root, false); e != nil {
			return e
		}
		hosts, _ := filepath.Glob(filepath.Join(kaspaAccessRoot(root), "sharing", "ManagerHost-*.exe"))
		for _, host := range hosts {
			if e := os.Remove(host); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
	}
	if e := chainPathGuard(root); e != nil {
		return e
	}
	if e := stopOne(root, "dual"); e != nil {
		return e
	}
	if !bridgeOnly {
		if e := stopOne(root, "kaspa"); e != nil {
			return e
		}
	}
	c, e := chains.Read(root)
	if e != nil {
		return e
	}
	if c.LAN {
		if e = chainFirewall(root, true); e != nil {
			return e
		}
	}
	if e = os.RemoveAll(filepath.Join(root, "components", "merged-"+chains.DualVersion)); e != nil {
		return e
	}
	c.DualBridge = chains.Binary{}
	if e = os.Remove(filepath.Join(root, "components", "kaspa-"+chains.KaspaVersion, "stratum-bridge.exe")); e != nil && !os.IsNotExist(e) {
		return e
	}
	c.KaspaBridge = chains.Binary{}
	if !bridgeOnly {
		if e = os.RemoveAll(filepath.Join(root, "components")); e != nil {
			return e
		}
		c.Kaspa = chains.Binary{}
		c.AutoStart = false
		c.Wallet = chains.Binary{}
		c.Host = ""
	}
	c.LAN = false
	return chains.Save(root, c)
}
