package main

import (
	"fmt"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"strings"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/lanshare"
	"zkas-node-manager/internal/node"
)

type lanUI struct {
	enabled              *walk.CheckBox
	ip                   *walk.ComboBox
	state                *walk.Label
	details              *walk.TextEdit
	start, stop, refresh *walk.PushButton
}

func (u *lanUI) read(s *node.SharingConfig) { s.LAN = u.enabled.Checked(); s.LANIP = u.ip.Text() }

func (m *manager) lanPanel(kaspa bool) Widget {
	u := &m.zLAN
	s := m.shared
	start := m.startSharingClicked
	root := m.root
	name := "ZKas"
	if kaspa {
		u = &m.kLAN
		s, _ = readKaspaSharing(m.root)
		start = m.startKaspaSharingClicked
		root = kaspaAccessRoot(m.root)
		name = "Kaspa"
	}
	ips := lanshare.Addresses()
	idx := -1
	for i, ip := range ips {
		if ip == s.LANIP {
			idx = i
		}
	}
	if idx < 0 && s.LANIP != "" {
		ips = append(ips, s.LANIP)
		idx = len(ips) - 1
	}
	if idx < 0 && len(ips) > 0 {
		idx = 0
	}
	return sharingPanel("LOCAL NETWORK / WI-FI", "Use "+name+" from another computer", "For bots and wallet apps on your home network. Choose this PC's Wi-Fi or Ethernet address, enable access, then start sharing. No domain or router port forwarding is needed.", walk.RGB(155, 87, 12), walk.RGB(255, 248, 231), []Widget{
		CheckBox{AssignTo: &u.enabled, Text: "Enable local-network access for " + name, Checked: s.LAN},
		Label{Text: "This PC's local IPv4 address"},
		ComboBox{AssignTo: &u.ip, Model: ips, CurrentIndex: idx},
		PushButton{AssignTo: &u.refresh, Text: "Refresh local addresses", OnClicked: func() {
			current := u.ip.Text()
			ips := lanshare.Addresses()
			u.ip.SetModel(ips)
			idx := -1
			for i, ip := range ips {
				if ip == current {
					idx = i
				}
			}
			if idx < 0 && len(ips) > 0 {
				idx = 0
			}
			u.ip.SetCurrentIndex(idx)
		}},
		TextLabel{MinSize: Size{Width: 240}, Text: "Use only on a trusted network: devices on the selected subnet can access the node without a sharing password. Connections are unencrypted. Windows must mark this network Private (or Domain); firewall approval may appear. Guest Wi-Fi isolation can block connections. Never forward these LAN ports in your router."},
		Label{AssignTo: &u.state, Font: Font{Bold: true}},
		PushButton{AssignTo: &u.start, Text: "Apply choices / start sharing", OnClicked: start},
		PushButton{AssignTo: &u.stop, Text: "Stop API sharing", OnClicked: func() {
			m.chainAction("Stopping "+name+" sharing", name+" LAN, HTTPS and Tor access stopped. Node and peer access are unchanged.", func() error { return stopOne(root, "sharing") })
		}},
		TextEdit{AssignTo: &u.details, ReadOnly: true, VScroll: true, MinSize: Size{Height: 125}},
		PushButton{Text: "Copy local connection details", OnClicked: func() { m.copyText(name+" LAN connections", u.details.Text()) }},
		TextLabel{MinSize: Size{Width: 240}, Text: "Apply uses all checked sharing options on this page. Stop closes LAN, HTTPS and Tor access. To turn off only LAN: stop sharing, uncheck LAN, then apply the other choices. On the other computer use the address above instead of localhost or 127.0.0.1. Reserve this PC's IP in your router if you want it to stay the same."},
	})
}

func (m *manager) refreshLAN(kaspa, active bool) {
	u := &m.zLAN
	s := m.shared
	ready := serviceActive(m.root, "node")
	name := "ZKas"
	if kaspa {
		u = &m.kLAN
		s, _ = readKaspaSharing(m.root)
		ready = serviceActive(m.root, "kaspa")
		name = "Kaspa"
	} else if m.cfg.EnableWallet {
		ready = ready && serviceActive(m.root, "wallet")
	}
	if u.state == nil {
		return
	}
	state := "Local-network access is off"
	if s.LAN {
		state = "Local-network access is configured, but sharing is stopped"
		if active {
			state = "Local-network listener is running — test from your other computer"
		}
	}
	u.state.SetText(state)
	details := "Enable local-network access, choose an address and apply to get connection details."
	if s.LAN {
		details = name + " on " + s.LANIP + " (" + state + ")\r\n"
		if kaspa {
			c, _ := chains.Read(m.root)
			details += fmt.Sprintf("gRPC: %s:%d\r\nWebSocket Borsh: ws://%s:%d\r\nWebSocket JSON: ws://%s:%d\r\n", s.LANIP, c.GRPC, s.LANIP, c.Borsh, s.LANIP, c.JSON)
			if c.NodeMode == "basic" {
				details += "Address/balance queries require Wallet / application backend in Settings → Kaspa.\r\n"
			}
		} else {
			c := m.cfg
			details += fmt.Sprintf("gRPC: %s:%d\r\n", s.LANIP, c.GRPC)
			if c.EnableWebSocket {
				details += fmt.Sprintf("WebSocket JSON: ws://%s:%d\r\n", s.LANIP, c.WebSocket)
			}
			if c.EnableWallet {
				details += fmt.Sprintf("Wallet REST API: http://%s:%d\r\nWallet requests still use their own X-Wallet-Token.\r\n", s.LANIP, c.WalletPort)
			}
		}
		details += "Use the protocol supported by your bot/SDK. Reachability from another device has not been tested."
		if _, e := lanshare.Subnet(s.LANIP); e != nil {
			details += "\r\nAddress changed: stop sharing, refresh addresses and apply again."
		}
	}
	if strings.TrimSpace(u.details.Text()) != strings.TrimSpace(details) {
		u.details.SetText(details)
	}
	for _, w := range []walk.Widget{u.enabled, u.ip, u.refresh} {
		w.SetEnabled(!m.busy && !active)
	}
	u.start.SetEnabled(!m.busy && !active && ready)
	u.start.SetVisible(!active)
	u.stop.SetEnabled(!m.busy && active)
	u.stop.SetVisible(active)
}
