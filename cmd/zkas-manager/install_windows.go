package main

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"zkas-node-manager/internal/node"
)

func rootDir() string         { return filepath.Join(os.Getenv("LOCALAPPDATA"), "ZKasNodeManager") }
func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func shortcutPath() string {
	return filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "ZKas Node Manager.lnk")
}
func ensureInstalled(root string) error {
	sourceNow, err := os.Executable()
	p, active, activeErr := activeManager(root)
	if activeErr != nil {
		return activeErr
	}
	if err == nil && active != "" && strings.EqualFold(sourceNow, active) {
		if _, err = walletRuntime(root); err != nil {
			return err
		}
		return refreshManagerLinks(root, active, p.Version)
	}
	if e := stageWalletRuntime(root); e != nil {
		return e
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		return e
	}
	source, e := os.Executable()
	if e != nil {
		return e
	}
	dest := filepath.Join(root, "ZKasNodeManager.exe")
	if !strings.EqualFold(source, dest) {
		// A manager upgrade must not overwrite the executable hosting a running node.
		srcHash, e := node.FileHash(source)
		if e != nil {
			return e
		}
		dstHash, _ := node.FileHash(dest)
		if srcHash != dstHash {
			if hostActive(root) {
				return fmt.Errorf("Stop the node before installing a different manager version")
			}
			b, e := os.ReadFile(source)
			if e != nil {
				return e
			}
			if e = os.WriteFile(dest+".new", b, 0700); e != nil {
				return e
			}
			if e = os.Rename(dest+".new", dest); e != nil {
				return fmt.Errorf("Close any other installed manager window and retry: %w", e)
			}
		}
	}
	script := "$s=(New-Object -ComObject WScript.Shell).CreateShortcut(" + psQuote(shortcutPath()) + ");$s.TargetPath=" + psQuote(dest) + ";$s.WorkingDirectory=" + psQuote(root) + ";$s.Description='Install and manage a local ZKas node';$s.Save()"
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if b, e := cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("Cannot create Start menu shortcut: %s %w", b, e)
	}
	key, _, e := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall\ZKasNodeManager`, registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer key.Close()
	for k, v := range map[string]string{"DisplayName": "ZKas Node Manager (Preview)", "DisplayVersion": node.ManagerVersion, "Publisher": "Independent community utility", "InstallLocation": root, "DisplayIcon": dest, "UninstallString": `"` + dest + `" --uninstall`} {
		if e = key.SetStringValue(k, v); e != nil {
			return e
		}
	}
	key.SetDWordValue("NoModify", 1)
	key.SetDWordValue("NoRepair", 1)
	return nil
}
func setAutoStart(root string, enabled bool) error {
	key, _, e := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer key.Close()
	if enabled {
		exe, err := preferredManagerExe(root)
		if err != nil {
			return err
		}
		return key.SetStringValue("ZKasNodeManager", `"`+exe+`" --autostart`)
	}
	e = key.DeleteValue("ZKasNodeManager")
	if e == registry.ErrNotExist {
		return nil
	}
	return e
}
func validateData(root string, c node.Config) error {
	if e := c.Validate(); e != nil {
		return e
	}
	for _, reserved := range []string{"manager-versions", "manager-updates"} {
		rel, err := filepath.Rel(filepath.Join(root, reserved), c.DataDir)
		if err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))) {
			return fmt.Errorf("Choose a data folder outside the manager's update folders")
		}
	}
	rel, e := filepath.Rel(filepath.Join(root, "versions"), c.DataDir)
	if e == nil && (rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..")) {
		return fmt.Errorf("Choose a data folder outside the manager's versions directory")
	}
	if strings.HasPrefix(c.DataDir, `\\`) {
		return fmt.Errorf("Use a local disk for the node database")
	}
	return nil
}
func uninstall(root string) error {
	if e := stopNode(root); e != nil {
		return e
	}
	c, e := node.ReadConfig(root)
	if e != nil {
		return e
	}
	if e = validateData(root, c); e != nil {
		return e
	}
	s, sharingErr := node.ReadSharing(root)
	if sharingErr != nil {
		return sharingErr
	}
	if c.PublicP2P || s.HTTPS {
		s.HostExecutable, e = sharingHostPath(root)
		if e != nil {
			return e
		}
		if e = runPS("$ErrorActionPreference='Stop';$p=Start-Process -FilePath " + psQuote(s.HostExecutable) + " -ArgumentList '--remove-sharing-firewall' -Verb RunAs -Wait -PassThru;if($p.ExitCode -ne 0){throw 'Unable to remove sharing firewall rules'}"); e != nil {
			return e
		}
	}
	s.HTTPS = false
	s.TorMode = "off"
	s.HostExecutable = ""
	c.PublicP2P = false
	if e = node.SaveSharing(root, s); e != nil {
		return e
	}
	// Preserve onion identities and certificates with the retained user data.
	if e = uninstallMining(root); e != nil {
		return e
	}
	if e = setAutoStart(root, false); e != nil {
		return e
	}
	if e = os.RemoveAll(filepath.Join(root, "versions")); e != nil {
		return e
	}
	os.Remove(shortcutPath())
	registry.DeleteKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall\ZKasNodeManager`)
	c.WalletExecutable = ""
	c.WalletSHA256 = ""
	c.Executable = ""
	c.SHA256 = ""
	c.Version = ""
	c.AutoStart = false
	if e = node.Save(root, c); e != nil {
		return e
	}
	// The current exe cannot delete itself on Windows. A temporary copy waits for
	// this UI to exit, deletes only the known installed exe, and leaves data/logs.
	source, e := os.Executable()
	if e != nil {
		return e
	}
	b, e := os.ReadFile(source)
	if e != nil {
		return e
	}
	helper := filepath.Join(os.TempDir(), fmt.Sprintf("ZKas-cleanup-%d.exe", time.Now().UnixNano()))
	if e = os.WriteFile(helper, b, 0700); e != nil {
		return e
	}
	cmd := exec.Command(helper, "--cleanup")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
func cleanup(root string) {
	os.Remove(filepath.Join(root, "active-manager.json"))
	for i := 0; i < 60; i++ {
		time.Sleep(time.Second)
		e := os.Remove(filepath.Join(root, "ZKasNodeManager.exe"))
		v := os.RemoveAll(filepath.Join(root, "manager-versions"))
		u := os.RemoveAll(filepath.Join(root, "manager-updates"))
		if (e == nil || os.IsNotExist(e)) && v == nil && u == nil {
			break
		}
	}
}
