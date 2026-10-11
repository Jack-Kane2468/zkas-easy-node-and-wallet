package lanshare

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestSubnetRestrictions(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("192.168.10.0/24")
	for addr, want := range map[string]bool{"192.168.10.2:44000": true, "192.168.11.2:44000": false, "8.8.8.8:44000": false, "127.0.0.1:44000": false, "[::1]:44000": false} {
		a, e := net.ResolveTCPAddr("tcp", addr)
		if e != nil {
			t.Fatal(e)
		}
		if got := Allowed(subnet, a); got != want {
			t.Errorf("%s: %v", addr, got)
		}
	}
	for _, address := range []string{"0.0.0.0", "127.0.0.1", "8.8.8.8", "::", "not an address"} {
		if _, e := Subnet(address); e == nil {
			t.Errorf("accepted %s", address)
		}
	}
}

func TestRelayAndShutdown(t *testing.T) {
	ips := []string{"127.0.0.2"}
	upstream, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer upstream.Close()
	port := upstream.Addr().(*net.TCPAddr).Port
	go func() {
		c, e := upstream.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		io.Copy(c, c)
	}()
	errors := make(chan error, 4)
	server, e := testStart(context.Background(), ips[0], []Route{{Port: port}}, errors)
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	c, e := net.DialTimeout("tcp4", net.JoinHostPort(ips[0], strconv.Itoa(port)), time.Second)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	if _, e = c.Write([]byte("node RPC payload")); e != nil {
		t.Fatal(e)
	}
	b := make([]byte, 16)
	if _, e = io.ReadFull(c, b); e != nil {
		t.Fatal(e)
	}
	if string(b) != "node RPC payload" {
		t.Fatalf("bad relay %q", b)
	}
	server.Close()
	if _, e = c.Read(b); e == nil {
		t.Fatal("Stop did not close existing connection")
	}
	if c, e := net.DialTimeout("tcp4", net.JoinHostPort(ips[0], strconv.Itoa(port)), time.Second); e == nil {
		c.Close()
		t.Fatal("Stop left listener open")
	}
	select {
	case e := <-errors:
		t.Fatal(e)
	default:
	}
}

func TestHTTPAndRollback(t *testing.T) {
	ips := []string{"127.0.0.2"}
	reserve, e := net.Listen("tcp4", net.JoinHostPort(ips[0], "0"))
	if e != nil {
		t.Fatal(e)
	}
	port := reserve.Addr().(*net.TCPAddr).Port
	reserve.Close()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("wallet gateway")) })
	s, e := testStart(context.Background(), ips[0], []Route{{Port: port, Handler: handler}}, make(chan error, 4))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
	r, e := client.Get("http://" + net.JoinHostPort(ips[0], strconv.Itoa(port)))
	if e != nil {
		t.Fatal(e)
	}
	b, e := io.ReadAll(r.Body)
	r.Body.Close()
	if e != nil || string(b) != "wallet gateway" {
		t.Fatalf("HTTP: %q %v", b, e)
	}
	free, e := net.Listen("tcp4", net.JoinHostPort(ips[0], "0"))
	if e != nil {
		t.Fatal(e)
	}
	first := free.Addr().(*net.TCPAddr).Port
	free.Close()
	if failed, e := testStart(context.Background(), ips[0], []Route{{Port: first}, {Port: port}}, make(chan error, 4)); e == nil {
		failed.Close()
		t.Fatal("Expected occupied-port failure")
	}
	freed, e := net.Listen("tcp4", net.JoinHostPort(ips[0], strconv.Itoa(first)))
	if e != nil {
		t.Fatal("Partial failure left a listener open:", e)
	}
	freed.Close()
}

// Exercise actual sockets on loopback while presenting a private-subnet client
// address to the same production filter. This does not certify Windows LAN access.
type privateConn struct{ net.Conn }

func (c privateConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("192.168.10.20"), Port: 12345}
}

type privateListener struct{ net.Listener }

func (l privateListener) Accept() (net.Conn, error) {
	c, e := l.Listener.Accept()
	if e != nil {
		return nil, e
	}
	return privateConn{c}, nil
}
func testStart(ctx context.Context, address string, routes []Route, errs chan<- error) (*Server, error) {
	_, subnet, _ := net.ParseCIDR("192.168.10.0/24")
	return start(ctx, address, subnet, routes, errs, func(network, address string) (net.Listener, error) {
		l, e := net.Listen(network, address)
		if e != nil {
			return nil, e
		}
		return privateListener{l}, nil
	})
}
