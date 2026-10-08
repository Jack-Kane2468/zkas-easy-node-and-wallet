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
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/node"
)

func runKaspaSharingHost(managerRoot string) error {
	root := kaspaAccessRoot(managerRoot)
	lock, e := mutex(`Local\ZKasNodeHost-` + serviceIdentity(root, "sharing"))
	if e != nil {
		return e
	}
	defer windows.CloseHandle(lock)
	s, e := node.ReadSharing(root)
	if e != nil {
		return e
	}
	kc, e := chains.Read(managerRoot)
	c := node.Config{EnableWallet: true, GRPC: kc.GRPC, WalletPort: kc.Borsh, WebSocket: kc.JSON}
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
		if e = launch(net.JoinHostPort("0.0.0.0", strconv.Itoa(s.Port)), chains.WRPCGateway(kc.Borsh, kc.JSON, s.Token, !s.PublicAPI), true); e != nil {
			return e
		}
	}
	var torReady atomic.Bool
	if s.TorMode != "off" {
		// Tor client auth protects personal mode before a circuit reaches the HTTP API.
		if e = launch("127.0.0.1:"+strconv.Itoa(kaspaOnionPort), chains.WRPCGateway(kc.Borsh, kc.JSON, "", false), false); e != nil {
			return e
		}
		conf, e := s.TorConfig(root)
		conf = strings.ReplaceAll(conf, "127.0.0.1:18502", "127.0.0.1:18503")
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
		defer log.Close()
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
