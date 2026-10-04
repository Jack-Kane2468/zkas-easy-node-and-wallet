package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lxn/walk"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"zkas-node-manager/internal/managerupdate"
	"zkas-node-manager/internal/node"
)

type managerPointer struct {
	Version   string `json:"version"`
	Directory string `json:"directory"`
	SHA256    string `json:"sha256"`
}
type managerUpdateJob struct {
	Version     string `json:"version"`
	Directory   string `json:"directory"`
	SHA256      string `json:"sha256"`
	ParentPID   uint32 `json:"parentPID"`
	PreviousExe string `json:"previousExe"`
}

func checkedManagerVersion(root string, p managerPointer) (string, error) {
	if _, e := managerupdate.ParseVersion(p.Version); e != nil {
		return "", e
	}
	if len(p.SHA256) != 64 || strings.ContainsAny(p.SHA256, "/\\") || p.Directory != p.Version+"-"+p.SHA256[:12] {
		return "", fmt.Errorf("invalid manager version folder")
	}
	dir := filepath.Join(root, "manager-versions", p.Directory)
	for _, d := range []string{filepath.Dir(dir), dir} {
		if st, e := os.Lstat(d); e != nil || st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
			return "", fmt.Errorf("manager folder missing or redirected")
		}
	}
	exe := filepath.Join(dir, "ZKasNodeManager.exe")
	h, e := node.FileHash(exe)
	if e != nil || h != p.SHA256 {
		return "", fmt.Errorf("selected manager checksum changed")
	}
	return exe, nil
}
func activeManager(root string) (managerPointer, string, error) {
	var p managerPointer
	b, e := os.ReadFile(filepath.Join(root, "active-manager.json"))
	if os.IsNotExist(e) {
		return p, "", nil
	}
	if e != nil {
		return p, "", e
	}
	if len(b) > 4096 {
		return p, "", fmt.Errorf("invalid manager selection")
	}
	if e = json.Unmarshal(b, &p); e != nil {
		return p, "", e
	}
	exe, e := checkedManagerVersion(root, p)
	return p, exe, e
}
func preferredManagerExe(root string) (string, error) {
	_, exe, e := activeManager(root)
	if e != nil {
		return "", e
	}
	if exe != "" {
		return exe, nil
	}
	return filepath.Join(root, "ZKasNodeManager.exe"), nil
}
func launchManager(exe string, args ...string) (*exec.Cmd, error) {
	c := exec.Command(exe, args...)
	c.Dir = filepath.Dir(exe)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if e := c.Start(); e != nil {
		return nil, e
	}
	return c, nil
}
func redirectToUpdatedManager(root string) bool {
	p, exe, e := activeManager(root)
	if e != nil {
		walk.MsgBox(nil, "Saved manager unavailable", e.Error()+"\nOpening this copy instead. Your data is unchanged.", walk.MsgBoxIconWarning)
		return false
	}
	if exe == "" {
		return false
	}
	cur, _ := managerupdate.ParseVersion(node.ManagerVersion)
	next, _ := managerupdate.ParseVersion(p.Version)
	if managerupdate.Compare(next, cur) <= 0 {
		return false
	}
	if _, e = launchManager(exe); e != nil {
		walk.MsgBox(nil, "Unable to open updated manager", e.Error(), walk.MsgBoxIconWarning)
		return false
	}
	return true
}
func (m *manager) checkManagerUpdate() {
	previews := m.managerPreviews.Checked()
	m.action(func() error {
		m.setProgress("Checking manager releases on GitHub…")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		client := managerupdate.HTTPClient()
		r, e := managerupdate.Latest(ctx, client, node.ManagerVersion, previews)
		if e != nil {
			return e
		}
		if r == nil {
			m.onUI(func() {
				walk.MsgBox(m.window, "Manager is current", "No newer manager release is available for your selected release channel.\nCurrent manager: "+node.ManagerVersion, walk.MsgBoxIconInformation)
			})
			return nil
		}
		notes := r.Body
		if len(notes) > 6000 {
			notes = notes[:6000] + "\n…"
		}
		yes := false
		m.onUI(func() {
			yes = walk.MsgBox(m.window, "Update manager", fmt.Sprintf("Update manager %s to %s?\n\nThe manager will close and reopen. The wallet vault will lock. Node, wallet backend, mining and sharing stay running. Settings and data are kept. The old manager is retained if startup fails.\n\nSource: github.com/%s\n\n%s", node.ManagerVersion, r.Tag, managerupdate.Repository, notes), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes
		})
		if !yes {
			return nil
		}
		jobs := filepath.Join(m.root, "manager-updates")
		if e = os.MkdirAll(jobs, 0700); e != nil {
			return e
		}
		jobdir, e := os.MkdirTemp(jobs, "update-")
		if e != nil {
			return e
		}
		m.setProgress("Downloading and verifying the manager package…")
		archive := filepath.Join(jobdir, "release.zip")
		if e = managerupdate.Download(ctx, client, *r, archive); e != nil {
			return e
		}
		stage := filepath.Join(jobdir, "package")
		if e = managerupdate.Extract(archive, stage); e != nil {
			return e
		}
		hash, e := node.FileHash(filepath.Join(stage, "ZKasNodeManager.exe"))
		if e != nil {
			return e
		}
		version := strings.TrimPrefix(r.Tag, "v")
		dir := version + "-" + hash[:12]
		versions := filepath.Join(m.root, "manager-versions")
		if e = os.MkdirAll(versions, 0700); e != nil {
			return e
		}
		dest := filepath.Join(versions, dir)
		if _, e = os.Stat(dest); os.IsNotExist(e) {
			if e = os.Rename(stage, dest); e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
		p := managerPointer{version, dir, hash}
		exe, e := checkedManagerVersion(m.root, p)
		if e != nil {
			return e
		}
		m.setProgress("Checking the new manager and wallet runtime…")
		checkCtx, stop := context.WithTimeout(ctx, 30*time.Second)
		defer stop()
		cmd := exec.CommandContext(checkCtx, exe, "--manager-self-test")
		cmd.Dir = dest
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		out, e := cmd.Output()
		if e != nil {
			return fmt.Errorf("new manager self-test failed; current version unchanged: %w", e)
		}
		var check struct {
			Version string `json:"version"`
			OK      bool   `json:"ok"`
		}
		if json.Unmarshal(out, &check) != nil || !check.OK || check.Version != version {
			return fmt.Errorf("new manager reported an unexpected version or invalid runtime")
		}
		previous, e := os.Executable()
		if e != nil {
			return e
		}
		job := managerUpdateJob{version, dir, hash, uint32(os.Getpid()), previous}
		if e = node.AtomicJSON(filepath.Join(jobdir, "job.json"), job); e != nil {
			return e
		}
		helper := filepath.Join(jobdir, "manager-updater.exe")
		b, e := os.ReadFile(previous)
		if e != nil {
			return e
		}
		if e = os.WriteFile(helper, b, 0700); e != nil {
			return e
		}
		if _, e = launchManager(helper, "--activate-manager-update", filepath.Base(jobdir)); e != nil {
			return e
		}
		for i := 0; i < 100; i++ {
			if _, e = os.Stat(filepath.Join(jobdir, "helper-ready")); e == nil {
				m.onUI(func() { m.busy = false; m.window.Close() })
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		os.WriteFile(filepath.Join(jobdir, "cancelled"), []byte("cancelled"), 0600)
		return fmt.Errorf("update helper did not become ready; manager remains open")
	})
}
func updateJobDir(root, id string) (string, error) {
	if !strings.HasPrefix(id, "update-") || filepath.Base(id) != id || strings.ContainsAny(id, "\\/:") {
		return "", fmt.Errorf("invalid update job")
	}
	return filepath.Join(root, "manager-updates", id), nil
}
func managerSelfTest(root string) error {
	if _, e := walletRuntime(root); e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": true, "version": node.ManagerVersion})
}
func markManagerReady(root, id string) {
	if dir, e := updateJobDir(root, id); e == nil {
		os.WriteFile(filepath.Join(dir, "manager-ready"), []byte(node.ManagerVersion), 0600)
	}
}
func activateManagerUpdate(root, id string) error {
	lock, e := mutex(`Local\ZKasManagerUpdate-` + identity(root))
	if e != nil {
		return fmt.Errorf("another manager update is active")
	}
	defer windows.CloseHandle(lock)
	dir, e := updateJobDir(root, id)
	if e != nil {
		return e
	}
	b, e := os.ReadFile(filepath.Join(dir, "job.json"))
	if e != nil {
		return e
	}
	var job managerUpdateJob
	if e = json.Unmarshal(b, &job); e != nil {
		return e
	}
	p := managerPointer{job.Version, job.Directory, job.SHA256}
	exe, e := checkedManagerVersion(root, p)
	if e != nil {
		return e
	}
	parent, e := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, job.ParentPID)
	if e != nil {
		return e
	}
	defer windows.CloseHandle(parent)
	buf := make([]uint16, 32768)
	n := uint32(len(buf))
	if e = windows.QueryFullProcessImageName(parent, 0, &buf[0], &n); e != nil {
		return e
	}
	if !strings.EqualFold(windows.UTF16ToString(buf[:n]), job.PreviousExe) {
		return fmt.Errorf("update parent identity changed")
	}
	if e = os.WriteFile(filepath.Join(dir, "helper-ready"), []byte("ready"), 0600); e != nil {
		return e
	}
	result, e := windows.WaitForSingleObject(parent, 60000)
	if e != nil || result != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("manager did not close; update cancelled")
	}
	if _, e = os.Stat(filepath.Join(dir, "cancelled")); e == nil {
		return fmt.Errorf("update cancelled")
	}
	os.Remove(filepath.Join(dir, "manager-ready"))
	next, e := launchManager(exe, "--manager-update-ready", id)
	if e != nil {
		return reopenPrevious(job.PreviousExe, e)
	}
	done := make(chan error, 1)
	go func() { done <- next.Wait() }()
	ready := false
	for i := 0; i < 300; i++ {
		if b, e := os.ReadFile(filepath.Join(dir, "manager-ready")); e == nil && string(b) == job.Version {
			ready = true
			break
		}
		select {
		case e := <-done:
			return reopenPrevious(job.PreviousExe, fmt.Errorf("updated manager exited before startup: %v", e))
		default:
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		next.Process.Kill()
		<-done
		return reopenPrevious(job.PreviousExe, fmt.Errorf("new manager did not become ready; previous manager reopened"))
	}
	if old, e := os.ReadFile(filepath.Join(root, "active-manager.json")); e == nil {
		if e = os.WriteFile(filepath.Join(dir, "previous-manager.json"), old, 0600); e != nil {
			return e
		}
	}
	if e = node.AtomicJSON(filepath.Join(root, "active-manager.json"), p); e != nil {
		return fmt.Errorf("new manager opened, but saving default version failed: %w", e)
	}
	if e = refreshManagerLinks(root, exe, p.Version); e != nil {
		return fmt.Errorf("manager updated, but shortcut/startup registration needs repair: %w", e)
	}
	os.Remove(filepath.Join(dir, "release.zip"))
	return os.WriteFile(filepath.Join(dir, "complete"), []byte(job.Version), 0600)
}
func reopenPrevious(exe string, cause error) error {
	_, e := launchManager(exe, "--manager-fallback")
	if e != nil {
		return fmt.Errorf("%v; previous manager could not reopen: %w", cause, e)
	}
	return cause
}
func refreshManagerLinks(root, exe, version string) error {
	if _, e := os.Stat(filepath.Join(root, "ZKasNodeManager.exe")); os.IsNotExist(e) {
		return nil
	}
	script := "$ErrorActionPreference='Stop';$s=(New-Object -ComObject WScript.Shell).CreateShortcut(" + psQuote(shortcutPath()) + ");$s.TargetPath=" + psQuote(exe) + ";$s.WorkingDirectory=" + psQuote(filepath.Dir(exe)) + ";$s.Description='ZKas Node Manager';$s.Save()"
	if e := runPS(script); e != nil {
		return e
	}
	key, _, e := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall\ZKasNodeManager`, registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer key.Close()
	for k, v := range map[string]string{"DisplayVersion": version, "DisplayIcon": exe, "UninstallString": `"` + exe + `" --uninstall`} {
		if e = key.SetStringValue(k, v); e != nil {
			return e
		}
	}
	c, e := node.ReadConfig(root)
	if e != nil {
		return e
	}
	return setAutoStart(root, c.AutoStart)
}
