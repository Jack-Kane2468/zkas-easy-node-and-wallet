package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
	"zkas-node-manager/internal/node"
)

func sharingHostPath(root string) (string, error) {
	source, e := os.Executable()
	if e != nil {
		return "", e
	}
	hash, e := node.FileHash(source)
	if e != nil {
		return "", e
	}
	dir := filepath.Join(root, "sharing")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	// Windows ignores POSIX 0600; explicitly protect tokens, onion keys and TLS keys.
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return "", e
	}
	if e = runPS("$ErrorActionPreference='Stop'; & icacls.exe " + psQuote(dir) + " /inheritance:r /grant:r " + psQuote("*"+user.User.Sid.String()+":(OI)(CI)F") + " '*S-1-5-18:(OI)(CI)F' | Out-Null; if($LASTEXITCODE -ne 0){throw 'Unable to protect sharing keys'}"); e != nil {
		return "", e
	}
	dest := filepath.Join(dir, "ManagerHost-"+node.ManagerVersion+"-"+hash[:12]+".exe")
	if h, _ := node.FileHash(dest); h == hash {
		return dest, nil
	}
	b, e := os.ReadFile(source)
	if e != nil {
		return "", e
	}
	return dest, os.WriteFile(dest, b, 0700)
}

const peerRule = "ZKasNodeManager-Peers-16811"
const apiRule = "ZKasNodeManager-HTTPS"

func sharingFirewallHelper() error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	root := filepath.Dir(filepath.Dir(exe))
	c, e := node.ReadConfig(root)
	if e != nil {
		return e
	}
	s, e := node.ReadSharing(root)
	if e != nil {
		return e
	}
	script := "$ErrorActionPreference='Stop';Get-NetFirewallRule -Name '" + peerRule + "','" + apiRule + "' -ErrorAction SilentlyContinue | Remove-NetFirewallRule;"
	if c.PublicP2P {
		if e = node.CheckBinary(c); e != nil {
			return e
		}
		script += "New-NetFirewallRule -Name '" + peerRule + "' -DisplayName 'ZKas public peers TCP 16811' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 16811 -Profile Any -RemoteAddress Any -Program " + psQuote(c.Executable) + " | Out-Null;"
	}
	if s.HTTPS {
		if s.Port < 1024 || s.Port > 65535 {
			return fmt.Errorf("invalid HTTPS port")
		}
		if s.HostExecutable != exe {
			return fmt.Errorf("sharing host mismatch")
		}
		script += "New-NetFirewallRule -Name '" + apiRule + "' -DisplayName 'ZKas HTTPS API' -Direction Inbound -Action Allow -Protocol TCP -LocalPort " + strconv.Itoa(s.Port) + " -Profile Any -RemoteAddress Any -Program " + psQuote(s.HostExecutable) + " | Out-Null;"
	}
	e = runPS(script)
	if e != nil {
		os.WriteFile(filepath.Join(root, "sharing", "firewall-error.txt"), []byte(e.Error()), 0600)
	}
	return e
}
func applySharingFirewall(s node.SharingConfig) error {
	return runPS("$ErrorActionPreference='Stop';$p=Start-Process -FilePath " + psQuote(s.HostExecutable) + " -ArgumentList '--sharing-firewall' -Verb RunAs -Wait -PassThru;if($p.ExitCode -ne 0){throw 'Firewall update failed or was declined. See sharing/firewall-error.txt.'}")
}
func startSharing(root string, c node.Config, s node.SharingConfig) error {
	if serviceActive(root, "sharing") {
		return fmt.Errorf("stop sharing before changing its settings")
	}
	if e := s.Validate(c); e != nil {
		return e
	}
	if !s.HTTPS && s.TorMode == "off" {
		return fmt.Errorf("select HTTPS or an onion mode first")
	}
	if !serviceActive(root, "wallet") {
		return fmt.Errorf("start the wallet API first")
	}
	if s.HTTPS {
		if _, _, _, e := s.EnsureCertificate(root); e != nil {
			return e
		}
	}
	if s.TorMode != "off" {
		conf, e := s.TorConfig(root)
		if e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(root, "sharing", "torrc"), []byte(conf), 0600); e != nil {
			return e
		}
	}
	oldSharing, e := node.ReadSharing(root)
	if e != nil {
		return e
	}
	if e := node.SaveSharing(root, s); e != nil {
		return e
	}
	if s.HTTPS || oldSharing.HTTPS {
		if e := applySharingFirewall(s); e != nil {
			return e
		}
	}
	cmd := exec.Command(s.HostExecutable, "--sharing-host")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NEW_CONSOLE}
	if e := cmd.Start(); e != nil {
		return e
	}
	go cmd.Wait()
	for i := 0; i < 40; i++ {
		time.Sleep(200 * time.Millisecond)
		r, e := serviceCommand(root, "sharing", "status")
		if e == nil && r.Running {
			return nil
		}
		if b, e := os.ReadFile(serviceErrorFile(root, "sharing")); e == nil {
			return fmt.Errorf("%s", b)
		}
	}
	return fmt.Errorf("sharing did not start; check sharing-host-error.txt")
}
func runSharingHost(root string) error {
	lock, e := mutex(`Local\ZKasNodeHost-` + serviceIdentity(root, "sharing"))
	if e != nil {
		return e
	}
	defer windows.CloseHandle(lock)
	s, e := node.ReadSharing(root)
	if e != nil {
		return e
	}
	c, e := node.ReadConfig(root)
	if e != nil {
		return e
	}
	if e = s.Validate(c); e != nil {
		return e
	}
	if !s.HTTPS && s.TorMode == "off" {
		return fmt.Errorf("sharing is disabled")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errs := make(chan error, 4)
	servers := []*http.Server{}
	launch := func(addr string, handler http.Handler, tlsOn bool) error {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
		cert, key := "", ""
		if tlsOn {
			cert, key, _, err = s.EnsureCertificate(root)
			if err != nil {
				ln.Close()
				return err
			}
		}
		servers = append(servers, server)
		go func() {
			var err error
			if tlsOn {
				err = server.ServeTLS(ln, cert, key)
			} else {
				err = server.Serve(ln)
			}
			if err != http.ErrServerClosed {
				errs <- err
			}
		}()
		return nil
	}
	defer func() {
		for _, srv := range servers {
			srv.Close()
		}
	}()
	if s.HTTPS {
		if e = launch(net.JoinHostPort("0.0.0.0", strconv.Itoa(s.Port)), node.GatewayHandler(c.WalletPort, s.Token, !s.PublicAPI), true); e != nil {
			return e
		}
	}
	var torReady atomic.Bool
	if s.TorMode != "off" {
		// Tor client auth protects personal mode before a circuit reaches the HTTP API.
		if e = launch("127.0.0.1:"+strconv.Itoa(node.OnionGatewayPort), node.GatewayHandler(c.WalletPort, "", false), false); e != nil {
			return e
		}
		conf, e := s.TorConfig(root)
		if e != nil {
			return e
		}
		torrc := filepath.Join(root, "sharing", "torrc")
		if e = os.WriteFile(torrc, []byte(conf), 0600); e != nil {
			return e
		}
		log, e := newLog(filepath.Join(root, "logs", "tor-console.log"))
		if e != nil {
			return e
		}
		defer log.file.Close()
		tor := exec.CommandContext(ctx, s.TorExecutable, "-f", torrc)
		tor.Dir = filepath.Dir(s.TorExecutable)
		writer := &torLogWriter{log: log, ready: &torReady}
		tor.Stdout = writer
		tor.Stderr = writer
		tor.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if e = tor.Start(); e != nil {
			return e
		}
		torDone := make(chan struct{})
		defer func() { cancel(); <-torDone }()
		go func() {
			err := tor.Wait()
			if ctx.Err() == nil {
				errs <- fmt.Errorf("Tor exited: %v; see logs/tor-console.log", err)
			}
			close(torDone)
		}()
	}
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return e
	}
	listener, e := winio.ListenPipe(pipeName(root, "sharing"), &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;" + user.User.Sid.String() + ")", InputBufferSize: 4096, OutputBufferSize: 4096})
	if e != nil {
		return e
	}
	defer listener.Close()
	var once sync.Once
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(3 * time.Second))
				var command string
				if json.NewDecoder(conn).Decode(&command) != nil {
					return
				}
				reply := hostReply{Running: true, PID: os.Getpid(), Version: node.ManagerVersion, TorReady: torReady.Load()}
				if command == "stop" {
					reply.Stopping = true
				}
				json.NewEncoder(conn).Encode(reply)
				if command == "stop" {
					once.Do(cancel)
				}
			}()
		}
	}()
	select {
	case <-ctx.Done():
		return nil
	case e := <-errs:
		return e
	}
}

func ensurePeerFirewall(root string, c node.Config) error {
	if !c.PublicP2P {
		return nil
	}
	check := "$r=Get-NetFirewallRule -Name '" + peerRule + "' -ErrorAction SilentlyContinue;if(-not $r -or $r.Enabled -ne 'True' -or $r.Action -ne 'Allow'){exit 1};$a=$r|Get-NetFirewallApplicationFilter;if($a.Program -ne " + psQuote(c.Executable) + "){exit 1}"
	if runPS(check) == nil {
		return nil
	}
	s, e := node.ReadSharing(root)
	if e != nil {
		return e
	}
	s.HostExecutable, e = sharingHostPath(root)
	if e != nil {
		return e
	}
	if e = node.SaveSharing(root, s); e != nil {
		return e
	}
	return runPS("$ErrorActionPreference='Stop';$p=Start-Process -FilePath " + psQuote(s.HostExecutable) + " -ArgumentList '--peer-firewall' -Verb RunAs -Wait -PassThru;if($p.ExitCode -ne 0){throw 'Peer firewall setup failed'}")
}
func peerFirewallHelper() error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	root := filepath.Dir(filepath.Dir(exe))
	c, e := node.ReadConfig(root)
	if e != nil {
		return e
	}
	if !c.PublicP2P {
		return nil
	}
	if e = node.CheckBinary(c); e != nil {
		return e
	}
	return runPS("$ErrorActionPreference='Stop';Get-NetFirewallRule -Name '" + peerRule + "' -ErrorAction SilentlyContinue | Remove-NetFirewallRule;New-NetFirewallRule -Name '" + peerRule + "' -DisplayName 'ZKas public peers TCP 16811' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 16811 -Profile Any -RemoteAddress Any -Program " + psQuote(c.Executable) + " | Out-Null")
}

type torLogWriter struct {
	mu    sync.Mutex
	log   *rotatingLog
	ready *atomic.Bool
	tail  string
}

func (w *torLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.tail += string(p)
	if strings.Contains(w.tail, "Bootstrapped 100%") {
		w.ready.Store(true)
	}
	if len(w.tail) > 4096 {
		w.tail = w.tail[len(w.tail)-4096:]
	}
	return w.log.Write(p)
}
