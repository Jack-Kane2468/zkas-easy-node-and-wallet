package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
	"zkas-node-manager/internal/node"
)

var kernel = windows.NewLazySystemDLL("kernel32.dll")
var ctrlEvent = kernel.NewProc("GenerateConsoleCtrlEvent")

func identity(root string) string {
	b := sha256.Sum256([]byte(strings.ToLower(root)))
	return hex.EncodeToString(b[:12])
}
func serviceIdentity(root, kind string) string {
	if kind == "sharing" {
		return identity(root + "/sharing-service")
	}
	if kind == "mining" {
		return identity(root + "/mining-service")
	}
	if kind == "wallet" {
		return identity(root + "/wallet-service")
	}
	return identity(root)
}
func pipeName(root, kind string) string {
	return `\\.\pipe\ZKasNodeManager-` + serviceIdentity(root, kind)
}
func serviceErrorFile(root, kind string) string {
	if kind == "sharing" {
		return filepath.Join(root, "sharing-host-error.txt")
	}
	if kind == "mining" {
		return filepath.Join(root, "mining-host-error.txt")
	}
	if kind == "wallet" {
		return filepath.Join(root, "wallet-host-error.txt")
	}
	return filepath.Join(root, "host-error.txt")
}
func hostActive(root string) bool {
	return serviceActive(root, "sharing") || serviceActive(root, "node") || serviceActive(root, "wallet") || serviceActive(root, "mining")
}
func mutex(name string) (windows.Handle, error) {
	p, e := windows.UTF16PtrFromString(name)
	if e != nil {
		return 0, e
	}
	h, e := windows.CreateMutex(nil, false, p)
	if e != nil {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return 0, e
	}
	return h, nil
}
func serviceActive(root, kind string) bool {
	h, e := mutex(`Local\ZKasNodeHost-` + serviceIdentity(root, kind))
	if e == windows.ERROR_ALREADY_EXISTS {
		return true
	}
	if e != nil {
		return true
	}
	windows.CloseHandle(h)
	return false
}

type hostReply struct {
	TorReady bool `json:"torReady,omitempty"`

	Running  bool   `json:"running"`
	Stopping bool   `json:"stopping"`
	PID      int    `json:"pid"`
	Version  string `json:"version"`
	Error    string `json:"error,omitempty"`
}

func hostCommand(root, command string) (hostReply, error) {
	return serviceCommand(root, "node", command)
}
func serviceCommand(root, kind, command string) (hostReply, error) {
	var reply hostReply
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, e := winio.DialPipeContext(ctx, pipeName(root, kind))
	if e != nil {
		return reply, e
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if e = json.NewEncoder(conn).Encode(command); e != nil {
		return reply, e
	}
	e = json.NewDecoder(conn).Decode(&reply)
	return reply, e
}
func stopNode(root string) error {
	if e := stopOne(root, "sharing"); e != nil {
		return e
	}
	if e := stopOne(root, "mining"); e != nil {
		return e
	}
	// Wallet checkpoints must flush while its upstream node is still available.
	if e := stopOne(root, "wallet"); e != nil {
		return e
	}
	return stopOne(root, "node")
}
func stopOne(root, kind string) error {
	if !serviceActive(root, kind) {
		return nil
	}
	reply, e := serviceCommand(root, kind, "stop")
	if e != nil {
		return fmt.Errorf("%s host is busy or unreachable; retry Stop shortly: %w", kind, e)
	}
	if reply.Error != "" {
		return errors.New(reply.Error)
	}
	for i := 0; i < 90; i++ {
		if !serviceActive(root, kind) {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("%s is still shutting down. No process was force-killed and no update was activated. Wait, then retry", kind)
}
func startNode(root string, c node.Config) error {
	if e := c.Validate(); e != nil {
		return e
	}
	if e := node.CheckBinary(c); e != nil {
		return e
	}
	if e := ensurePeerFirewall(root, c); e != nil {
		return e
	}
	started := false
	if !serviceActive(root, "node") {
		if e := startOne(root, c, "node"); e != nil {
			return e
		}
		started = true
	}
	if c.EnableWallet && !serviceActive(root, "wallet") {
		if e := startOne(root, c, "wallet"); e != nil {
			return fmt.Errorf("Node is running, but wallet backend failed: %w", e)
		}
		started = true
	}
	if !started {
		return errors.New("Selected services are already running or starting")
	}
	return nil
}
func checkServicePorts(c node.Config, kind string) error {
	if kind == "mining" {
		if e := node.CheckMiningPort(); e != nil {
			return e
		}
		return node.CheckTCPPorts(node.MiningStatsPort)
	}
	if kind == "wallet" {
		return node.CheckTCPPorts(c.WalletPort)
	}
	c.EnableWallet = false
	return node.CheckPorts(c)
}
func startOne(root string, c node.Config, kind string) error {
	if serviceActive(root, kind) {
		return fmt.Errorf("%s is already running", kind)
	}
	if e := checkServicePorts(c, kind); e != nil {
		return e
	}
	for _, dir := range []string{c.DataDir, filepath.Join(root, "logs"), filepath.Join(root, "wallets")} {
		if e := os.MkdirAll(dir, 0700); e != nil {
			return e
		}
	}
	os.Remove(serviceErrorFile(root, kind))
	arg := "--host"
	if kind == "wallet" {
		arg = "--wallet-host"
	}
	hostExe := filepath.Join(root, "ZKasNodeManager.exe")
	if kind == "mining" {
		mc, e := node.ReadMining(root)
		if e != nil {
			return e
		}
		hostExe = mc.HostExecutable
		arg = "--mining-host"
	}
	cmd := exec.Command(hostExe, arg)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
	if e := cmd.Start(); e != nil {
		return e
	}
	go cmd.Wait()
	for i := 0; i < 50; i++ {
		time.Sleep(200 * time.Millisecond)
		r, e := serviceCommand(root, kind, "status")
		if e == nil && r.Running {
			time.Sleep(2 * time.Second)
			r, e = serviceCommand(root, kind, "status")
			if e == nil && r.Running {
				return nil
			}
			break
		}
		if b, e := os.ReadFile(serviceErrorFile(root, kind)); e == nil {
			return errors.New(string(b))
		}
	}
	b, _ := os.ReadFile(serviceErrorFile(root, kind))
	return fmt.Errorf("%s did not stay running. %s Check the logs", kind, string(b))
}

type rotatingLog struct {
	mu   sync.Mutex
	path string
	file *os.File
	size int64
}

func newLog(path string) (*rotatingLog, error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	s, e := f.Stat()
	if e != nil {
		f.Close()
		return nil, e
	}
	return &rotatingLog{path: path, file: f, size: s.Size()}, nil
}
func (l *rotatingLog) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.size+int64(len(b)) > 10<<20 {
		l.file.Close()
		os.Remove(l.path + ".1")
		if e := os.Rename(l.path, l.path+".1"); e != nil {
			return 0, e
		}
		f, e := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY, 0600)
		if e != nil {
			return 0, e
		}
		l.file = f
		l.size = 0
	}
	n, e := l.file.Write(b)
	l.size += int64(n)
	return n, e
}
func runHost(root, kind string) (result error) {
	// A GUI-subsystem executable must explicitly ensure it owns a console before
	// spawning a console child and delivering CTRL_C in this isolated console.
	console, _, _ := kernel.NewProc("GetConsoleWindow").Call()
	if console == 0 {
		ok, _, err := kernel.NewProc("AllocConsole").Call()
		if ok == 0 {
			return fmt.Errorf("Cannot allocate node console: %v", err)
		}
		console, _, _ = kernel.NewProc("GetConsoleWindow").Call()
	}
	if console != 0 {
		windows.NewLazySystemDLL("user32.dll").NewProc("ShowWindow").Call(console, 0)
	}

	defer func() {
		if result != nil {
			os.WriteFile(serviceErrorFile(root, kind), []byte(result.Error()), 0600)
		}
	}()
	lock, e := mutex(`Local\ZKasNodeHost-` + serviceIdentity(root, kind))
	if e != nil {
		return e
	}
	defer windows.CloseHandle(lock)
	c, e := node.ReadConfig(root)
	if e != nil {
		return e
	}
	if kind == "mining" {
		mc, err := node.ReadMining(root)
		if err != nil {
			return err
		}
		if err = node.CheckMiningBinary(mc); err != nil {
			return err
		}
	} else {
		if e = node.CheckBinary(c); e != nil {
			return e
		}
	}
	if e = checkServicePorts(c, kind); e != nil {
		return e
	}
	os.MkdirAll(filepath.Join(root, "logs"), 0700)
	logName := "console.log"
	if kind == "mining" {
		logName = "mining-console.log"
	}
	if kind == "wallet" {
		logName = "wallet-console.log"
	}
	log, e := newLog(filepath.Join(root, "logs", logName))
	if e != nil {
		return e
	}
	defer func() { log.file.Close() }()
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return e
	}
	listener, e := winio.ListenPipe(pipeName(root, kind), &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;" + user.User.Sid.String() + ")", InputBufferSize: 4096, OutputBufferSize: 4096})
	if e != nil {
		return e
	}
	defer listener.Close()
	exe, args := c.Executable, c.Args(root)
	if kind == "wallet" {
		if !c.EnableWallet {
			return errors.New("wallet backend is disabled")
		}
		exe, args = c.WalletExecutable, c.WalletArgs(root)
	}
	if kind == "mining" {
		mc, err := node.ReadMining(root)
		if err != nil {
			return err
		}
		exe, args = mc.Executable, mc.Args(root, c.GRPC)
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	cmd.Stdout = log
	cmd.Stderr = log
	// Each host owns a separate hidden console. A custom host handler ignores
	// CTRL_C without setting the inheritable ignore attribute; its child handles
	// CTRL_C normally (walletd uses Tokio ctrl_c to flush checkpoints).
	handler := syscall.NewCallback(func(event uint32) uintptr { return 1 })
	ok, _, controlErr := kernel.NewProc("SetConsoleCtrlHandler").Call(handler, 1)
	if ok == 0 {
		return fmt.Errorf("register console handler: %v", controlErr)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	// Do not let inherited KASPAD_* settings override the manager's selected network/settings.
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(entry), "KASPAD_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	if e = cmd.Start(); e != nil {
		return e
	}
	var mu sync.Mutex
	stopping := false
	go func() {
		for {
			conn, e := listener.Accept()
			if e != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(3 * time.Second))
				var request string
				if json.NewDecoder(bufio.NewReader(conn)).Decode(&request) != nil {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				r := hostReply{Running: true, PID: cmd.Process.Pid, Version: c.Version, Stopping: stopping}
				if request == "stop" && !stopping {
					ok, _, err := ctrlEvent.Call(windows.CTRL_C_EVENT, 0)
					if ok == 0 {
						r.Error = err.Error()
					} else {
						stopping = true
						r.Stopping = true
					}
				}
				json.NewEncoder(conn).Encode(r)
			}(conn)
		}
	}()
	e = cmd.Wait()
	mu.Lock()
	wasStopping := stopping
	mu.Unlock()
	if e != nil {
		return fmt.Errorf("%s exited: %w. See logs/%s", kind, e, logName)
	}
	if !wasStopping {
		return fmt.Errorf("%s exited unexpectedly; see logs/%s", kind, logName)
	}
	return nil
}
