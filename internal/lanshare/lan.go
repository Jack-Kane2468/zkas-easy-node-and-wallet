// Package lanshare exposes selected loopback services to one directly connected
// private IPv4 subnet. It never binds wildcard addresses or changes node flags.
package lanshare

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

func Addresses() []string {
	var out []string
	interfaces, _ := net.Interfaces()
	for _, i := range interfaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			ip, _, e := net.ParseCIDR(a.String())
			if e == nil && ip.To4() != nil && ip.IsPrivate() {
				out = append(out, ip.String())
			}
		}
	}
	sort.Strings(out)
	return out
}

func Subnet(address string) (*net.IPNet, error) {
	ip := net.ParseIP(address)
	if ip == nil || ip.To4() == nil || !ip.IsPrivate() {
		return nil, fmt.Errorf("Choose this PC's private IPv4 address (usually 192.168.x.x or 10.x.x)")
	}
	interfaces, e := net.Interfaces()
	if e != nil {
		return nil, e
	}
	for _, i := range interfaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			local, n, e := net.ParseCIDR(a.String())
			if e == nil && local.Equal(ip) {
				return n, nil
			}
		}
	}
	return nil, fmt.Errorf("The selected LAN address is no longer assigned to this PC. Select its current Wi-Fi or Ethernet address")
}

func Allowed(subnet *net.IPNet, remote net.Addr) bool {
	host, _, e := net.SplitHostPort(remote.String())
	if e != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.To4() != nil && ip.IsPrivate() && subnet.Contains(ip)
}

type Route struct {
	Port    int
	Handler http.Handler
}

// A nil Handler relays TCP to the same port on loopback (gRPC or WebSocket).
type Server struct {
	mu          sync.Mutex
	closed      bool
	listeners   []net.Listener
	connections map[net.Conn]bool
	slots       chan struct{}
}

func (s *Server) track(c net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		c.Close()
		return false
	}
	s.connections[c] = true
	return true
}
func (s *Server) untrack(c net.Conn) { c.Close(); s.mu.Lock(); delete(s.connections, c); s.mu.Unlock() }
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	for _, l := range s.listeners {
		l.Close()
	}
	for c := range s.connections {
		c.Close()
	}
	return nil
}

type filteredListener struct {
	net.Listener
	subnet *net.IPNet
	owner  *Server
}
type trackedConn struct {
	net.Conn
	owner *Server
	once  sync.Once
}

func (c *trackedConn) Close() error {
	c.once.Do(func() { c.owner.untrack(c.Conn); <-c.owner.slots })
	return nil
}
func (l *filteredListener) Accept() (net.Conn, error) {
	for {
		c, e := l.Listener.Accept()
		if e != nil {
			return nil, e
		}
		if !Allowed(l.subnet, c.RemoteAddr()) {
			c.Close()
			continue
		}
		select {
		case l.owner.slots <- struct{}{}:
		default:
			c.Close()
			continue
		}
		if !l.owner.track(c) {
			<-l.owner.slots
			return nil, net.ErrClosed
		}
		return &trackedConn{Conn: c, owner: l.owner}, nil
	}
}
func Start(ctx context.Context, address string, routes []Route, errors chan<- error) (*Server, error) {
	subnet, e := Subnet(address)
	if e != nil {
		return nil, e
	}
	return start(ctx, address, subnet, routes, errors, net.Listen)
}

func start(ctx context.Context, address string, subnet *net.IPNet, routes []Route, errors chan<- error, listen func(string, string) (net.Listener, error)) (*Server, error) {
	s := &Server{connections: map[net.Conn]bool{}, slots: make(chan struct{}, 128)}
	for _, r := range routes {
		if r.Port < 1024 || r.Port > 65535 {
			s.Close()
			return nil, fmt.Errorf("Invalid LAN port %d", r.Port)
		}
		l, e := listen("tcp4", net.JoinHostPort(address, strconv.Itoa(r.Port)))
		if e != nil {
			s.Close()
			return nil, fmt.Errorf("LAN port %d: %w", r.Port, e)
		}
		s.listeners = append(s.listeners, &filteredListener{Listener: l, subnet: subnet, owner: s})
	}
	report := func(e error) {
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()
		if !closed && ctx.Err() == nil {
			select {
			case errors <- e:
			default:
			}
		}
	}
	for i, r := range routes {
		l := s.listeners[i]
		if r.Handler != nil {
			server := &http.Server{Handler: r.Handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
			go func() { report(server.Serve(l)) }()
		} else {
			go func() {
				for {
					c, e := l.Accept()
					if e != nil {
						report(e)
						return
					}
					go s.relay(ctx, c, r.Port)
				}
			}()
		}
	}
	return s, nil
}
func (s *Server) relay(ctx context.Context, client net.Conn, port int) {
	defer client.Close()
	d := net.Dialer{Timeout: 5 * time.Second}
	upstream, e := d.DialContext(ctx, "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if e != nil {
		return
	}
	if !s.track(upstream) {
		return
	}
	defer s.untrack(upstream)
	done := make(chan struct{})
	go func() { io.Copy(upstream, client); upstream.Close(); client.Close(); close(done) }()
	io.Copy(client, upstream)
	client.Close()
	upstream.Close()
	<-done
}
