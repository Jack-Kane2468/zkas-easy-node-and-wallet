package main

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/node"
)

const kaspaOnionPort = 18503
const kaspaPeerRule = "ZKasNodeManager-Kaspa-Peers"
const kaspaAPIRule = "ZKasNodeManager-Kaspa-WSS"

func kaspaAccessRoot(root string) string { return filepath.Join(root, "kaspa-access") }
func readKaspaSharing(root string) (node.SharingConfig, error) {
	k := kaspaAccessRoot(root)
	s, e := node.ReadSharing(k)
	if _, err := os.Stat(filepath.Join(k, "sharing", "config.json")); os.IsNotExist(err) {
		s.Port = 8444
	}
	return s, e
}
func validateKaspaSharing(root string, s node.SharingConfig) error {
	c, e := chains.Read(root)
	if e != nil {
		return e
	}
	if e = c.ValidateNode(); e != nil {
		return e
	}
	if c.NodeMode == "basic" && (s.HTTPS || s.TorMode != "off") {
		return fmt.Errorf("Choose Wallet / application backend or Archive in Settings → Kaspa before sharing with wallets")
	}
	fake := node.Config{EnableWallet: true, GRPC: c.GRPC, WalletPort: c.Borsh, WebSocket: c.JSON}
	if e = s.Validate(fake); e != nil {
		return e
	}
	z, e := node.ReadConfig(root)
	if e != nil {
		return e
	}
	zs, e := node.ReadSharing(root)
	if e != nil {
		return e
	}
	if s.HTTPS {
		for _, p := range []int{c.P2P, 18503, 18889, 18081, 18115, z.GRPC, z.WebSocket, z.WalletPort} {
			if s.Port == p {
				return fmt.Errorf("Kaspa HTTPS port conflicts with an existing local service")
			}
		}
		if zs.HTTPS && s.Port == zs.Port {
			return fmt.Errorf("Kaspa and ZKas HTTPS need different ports")
		}
	}
	return nil
}
func startKaspaSharing(root string, s node.SharingConfig) error {
	k := kaspaAccessRoot(root)
	if serviceActive(k, "sharing") {
		return fmt.Errorf("Stop Kaspa sharing before changing choices")
	}
	if !serviceActive(root, "kaspa") {
		return fmt.Errorf("Start Kaspa in Overview first")
	}
	if !s.HTTPS && s.TorMode == "off" {
		return fmt.Errorf("Choose HTTPS or Tor first")
	}
	if e := validateKaspaSharing(root, s); e != nil {
		return e
	}
	var e error
	s.HostExecutable, e = sharingHostPath(k)
	if e != nil {
		return e
	}
	if s.Token == "" {
		s.Token, e = node.NewSharingToken()
		if e != nil {
			return e
		}
	}
	if s.HTTPS {
		if _, _, _, e = s.EnsureCertificate(k); e != nil {
			return e
		}
	}
	previous, e := readKaspaSharing(root)
	if e != nil {
		return e
	}
	if e = node.SaveSharing(k, s); e != nil {
		return e
	}
	if s.HTTPS || previous.HTTPS {
		if e = applyKaspaFirewall(root, false); e != nil {
			return e
		}
	}
	os.MkdirAll(filepath.Join(k, "logs"), 0700)
	os.Remove(serviceErrorFile(k, "sharing"))
	cmd := exec.Command(s.HostExecutable, "--kaspa-sharing-host")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NEW_CONSOLE}
	if e = cmd.Start(); e != nil {
		return e
	}
	go cmd.Wait()
	for i := 0; i < 40; i++ {
		time.Sleep(200 * time.Millisecond)
		r, e := serviceCommand(k, "sharing", "status")
		if e == nil && r.Running {
			return nil
		}
		if b, e := os.ReadFile(serviceErrorFile(k, "sharing")); e == nil {
			return fmt.Errorf("%s", b)
		}
	}
	return fmt.Errorf("Kaspa sharing did not start. Check its Tor log and sharing host error")
}
func applyKaspaFirewall(root string, remove bool) error {
	exe, e := chainHostPath(root)
	if e != nil {
		return e
	}
	arg := "--kaspa-firewall"
	if remove {
		arg = "--remove-kaspa-firewall"
	}
	return runPS("$ErrorActionPreference='Stop';$p=Start-Process -FilePath " + psQuote(exe) + " -ArgumentList " + psQuote(arg) + " -Verb RunAs -Wait -PassThru;if($p.ExitCode -ne 0){throw 'Kaspa firewall update failed or was declined'}")
}
func ensureKaspaPeerFirewall(root string, c chains.Config) error {
	if !c.PublicP2P {
		return nil
	}
	check := "$r=Get-NetFirewallRule -Name '" + kaspaPeerRule + "' -ErrorAction SilentlyContinue;if(-not $r -or $r.Enabled -ne 'True'){exit 1};$a=$r|Get-NetFirewallApplicationFilter;$p=$r|Get-NetFirewallPortFilter;if($a.Program -ne " + psQuote(c.Kaspa.Path) + " -or $p.LocalPort -ne '" + strconv.Itoa(c.P2P) + "'){exit 1}"
	if runPS(check) == nil {
		return nil
	}
	return applyKaspaFirewall(root, false)
}
func removeKaspaAccessRules(root string) error {
	c, e := chains.Read(root)
	if e != nil {
		return e
	}
	s, e := readKaspaSharing(root)
	if e != nil {
		return e
	}
	if !c.PublicP2P && !s.HTTPS {
		return nil
	}
	if e = applyKaspaFirewall(root, true); e != nil {
		return e
	}
	s.HTTPS = false
	s.TorMode = "off"
	if e = node.SaveSharing(kaspaAccessRoot(root), s); e != nil {
		return e
	}
	c.PublicP2P = false
	return chains.Save(root, c)
}
func kaspaFirewallHelper(remove bool) error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	root := filepath.Dir(filepath.Dir(exe))
	c, e := chains.Read(root)
	if e != nil {
		return e
	}
	s, e := readKaspaSharing(root)
	if e != nil {
		return e
	}
	script := "$ErrorActionPreference='Stop';Get-NetFirewallRule -Name '" + kaspaPeerRule + "','" + kaspaAPIRule + "' -ErrorAction SilentlyContinue | Remove-NetFirewallRule;"
	if !remove {
		if e = c.ValidateNode(); e != nil {
			return e
		}
		if c.PublicP2P {
			if e = c.Kaspa.Check(); e != nil {
				return e
			}
			script += "New-NetFirewallRule -Name '" + kaspaPeerRule + "' -DisplayName 'Kaspa public peers' -Direction Inbound -Action Allow -Protocol TCP -LocalPort " + strconv.Itoa(c.P2P) + " -Profile Any -RemoteAddress Any -Program " + psQuote(c.Kaspa.Path) + " | Out-Null;"
		}
		if s.HTTPS {
			if e = validateKaspaSharing(root, s); e != nil {
				return e
			}
			script += "New-NetFirewallRule -Name '" + kaspaAPIRule + "' -DisplayName 'Kaspa secure WebSocket API' -Direction Inbound -Action Allow -Protocol TCP -LocalPort " + strconv.Itoa(s.Port) + " -Profile Any -RemoteAddress Any -Program " + psQuote(s.HostExecutable) + " | Out-Null;"
		}
	}
	return runPS(script)
}
