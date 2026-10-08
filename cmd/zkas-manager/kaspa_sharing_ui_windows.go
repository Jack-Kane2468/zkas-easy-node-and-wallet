package main

import (
	"fmt"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"path/filepath"
	"strings"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/node"
)

type kaspaSharingUI struct {
	kPeer, kHTTPS, kPublic              *walk.CheckBox
	kHost, kTorExe, kCert, kKey         *walk.LineEdit
	kSharePort                          *walk.NumberEdit
	kTorMode                            *walk.ComboBox
	kShareState                         *walk.Label
	kShareDetails                       *walk.TextEdit
	kShareStart, kShareStop, kPeerApply *walk.PushButton
}

func (m *manager) kaspaSharingTab() TabPage {
	c, _ := chains.Read(m.root)
	s, _ := readKaspaSharing(m.root)
	idx := 0
	if s.TorMode == "personal" {
		idx = 1
	} else if s.TorMode == "public" {
		idx = 2
	}
	pick := func(field **walk.LineEdit, filter string) {
		d := walk.FileDialog{Title: "Choose file", Filter: filter}
		if ok, _ := d.ShowOpen(m.window); ok {
			(*field).SetText(d.FilePath)
		}
	}
	return TabPage{Title: "Kaspa", Layout: VBox{MarginsZero: true}, Children: []Widget{ScrollView{Layout: VBox{Spacing: 18}, Children: []Widget{
		Label{Text: "What do you want to share?", Font: Font{PointSize: 16, Bold: true}}, TextLabel{Text: "These are separate choices, not steps. Peer sharing connects other nodes; wallet/app sharing exposes Kaspa WebSocket RPC. Your wallet keys stay on your device.", MinSize: Size{Width: 240}}, Label{AssignTo: &m.kShareState},
		sharingPanel("OTHER NODES", "Help other Kaspa nodes sync", "Peer sharing does not expose a wallet or miner connection.", walk.RGB(29, 78, 140), walk.RGB(236, 244, 255), []Widget{
			CheckBox{AssignTo: &m.kPeer, Text: "Allow other Kaspa nodes to connect", Checked: c.PublicP2P},
			PushButton{AssignTo: &m.kPeerApply, Text: "Save node-sharing choice", OnClicked: m.saveKaspaPeers},
			TextLabel{Text: "Stop Kaspa, save this choice, then start it. Forward the Kaspa peer TCP port shown in Settings from your router to this PC. Share PUBLIC_IP:PORT or domain:PORT, without https://.", MinSize: Size{Width: 240}},
		}),
		sharingPanel("WALLETS · INTERNET", "Connect apps using secure WebSockets", "Requires a public IP/domain and router port forwarding. Compatible clients can query the node and submit signed transactions.", walk.RGB(18, 104, 75), walk.RGB(235, 249, 241), []Widget{
			CheckBox{AssignTo: &m.kHTTPS, Text: "Enable HTTPS / secure WebSocket access", Checked: s.HTTPS}, Label{Text: "Public IP or domain (no URL prefix)"}, LineEdit{AssignTo: &m.kHost, Text: s.Host},
			Composite{Layout: HBox{}, Children: []Widget{Label{Text: "HTTPS port"}, NumberEdit{AssignTo: &m.kSharePort, MinValue: 1024, MaxValue: 65535, Decimals: 0, Value: float64(s.Port)}}},
			CheckBox{AssignTo: &m.kPublic, Text: "Public access: let other people connect", Checked: s.PublicAPI},
			TextLabel{Text: "Private access needs Authorization: Bearer TOKEN during the WebSocket handshake. Your SDK must support custom headers; otherwise use personal Tor access. Public access needs no token. Forward the HTTPS port to this PC; gRPC stays local.", MinSize: Size{Width: 240}},
			Composite{Layout: HBox{}, Children: []Widget{LineEdit{AssignTo: &m.kCert, Text: s.CertFile}, PushButton{Text: "Certificate…", OnClicked: func() { pick(&m.kCert, "PEM (*.pem)|*.pem") }}}},
			Composite{Layout: HBox{}, Children: []Widget{LineEdit{AssignTo: &m.kKey, Text: s.KeyFile}, PushButton{Text: "Private key…", OnClicked: func() { pick(&m.kKey, "PEM / key|*.pem;*.key") }}}},
			TextLabel{Text: "Blank certificate fields generate a self-signed certificate that clients must explicitly trust. For ordinary public clients, use a CA-issued certificate for your domain. Certificates are not automatically renewed.", MinSize: Size{Width: 240}},
		}),
		sharingPanel("WALLETS · TOR", "Connect using an onion address", "Tor creates the address; a domain and router forwarding are not required.", walk.RGB(108, 54, 150), walk.RGB(246, 239, 252), []Widget{
			ComboBox{AssignTo: &m.kTorMode, Model: []string{"Off", "Personal onion — client key required", "Public onion — share the URL"}, CurrentIndex: idx},
			Composite{Layout: HBox{}, Children: []Widget{LineEdit{AssignTo: &m.kTorExe, Text: s.TorExecutable}, PushButton{Text: "Select tor.exe…", OnClicked: func() { pick(&m.kTorExe, "Tor (tor.exe)|tor.exe") }}}},
			PushButton{Text: "Open official Tor download page", OnClicked: func() { openExternal("https://www.torproject.org/download/tor/") }},
			TextLabel{Text: "Use the Tor Expert Bundle with its supporting DLLs. Clients need Tor support. Personal mode requires client authorization. This protects incoming wallet/app connections; Kaspa's outbound peer connections still use the ordinary internet.", MinSize: Size{Width: 240}},
		}),
		sharingPanel("APPLY / CONNECT", "Kaspa wallet/app sharing", "These controls apply HTTPS and Tor; peer access uses the separate Save button above.", walk.RGB(55, 65, 81), walk.RGB(242, 244, 247), []Widget{
			PushButton{AssignTo: &m.kShareStart, Text: "Apply choices / start Kaspa sharing", OnClicked: m.startKaspaSharingClicked},
			PushButton{AssignTo: &m.kShareStop, Text: "Stop Kaspa HTTPS / Tor access", OnClicked: func() {
				m.chainAction("Stopping Kaspa sharing", "Kaspa HTTPS / Tor stopped. Peer access is unchanged.", func() error { return stopOne(kaspaAccessRoot(m.root), "sharing") })
			}},
			TextEdit{AssignTo: &m.kShareDetails, ReadOnly: true, VScroll: true, MinSize: Size{Height: 130}},
			PushButton{Text: "Copy connection details", OnClicked: func() { m.copyText("Kaspa sharing", m.kShareDetails.Text()) }},
			PushButton{Text: "Copy private HTTPS access token", OnClicked: func() {
				s, _ := readKaspaSharing(m.root)
				if s.Token != "" {
					m.copyText("Kaspa access token", s.Token)
				} else {
					m.walletError(fmt.Errorf("Start private HTTPS access first"))
				}
			}},
			PushButton{Text: "Copy personal onion credentials", OnClicked: func() {
				s, _ := readKaspaSharing(m.root)
				v, e := s.OnionClientCredentials(kaspaAccessRoot(m.root))
				if e != nil {
					m.walletError(e)
				} else {
					m.copyText("Kaspa onion client credentials", v)
				}
			}},
			TextLabel{Text: "Put personal onion credentials in a .auth_private file in your Tor client's ClientOnionAuthDir. Keep the key private. Saved URLs do not prove that your ISP/router permits access; test from another connection. CGNAT may require Tor or a public IP from your ISP.", MinSize: Size{Width: 240}},
			PushButton{Text: "Open Kaspa sharing files (keep keys private)", OnClicked: func() { m.open(filepath.Join(kaspaAccessRoot(m.root), "sharing")) }},
		}),
	}}}}
}
func (m *manager) saveKaspaPeers() {
	enabled := m.kPeer.Checked()
	m.chainAction("Saving Kaspa peer access", "Peer sharing saved. Start Kaspa in Overview.", func() error {
		if serviceActive(m.root, "kaspa") {
			return fmt.Errorf("Stop Kaspa first")
		}
		c, e := chains.Read(m.root)
		if e != nil {
			return e
		}
		if !componentPresent(c.Kaspa) {
			return fmt.Errorf("Install Kaspa first")
		}
		c.PublicP2P = enabled
		if e = chains.Save(m.root, c); e != nil {
			return e
		}
		return applyKaspaFirewall(m.root, false)
	})
}
func (m *manager) startKaspaSharingClicked() {
	s, e := readKaspaSharing(m.root)
	if e != nil {
		m.walletError(e)
		return
	}
	s.HTTPS = m.kHTTPS.Checked()
	s.PublicAPI = m.kPublic.Checked()
	s.Host = strings.TrimSpace(m.kHost.Text())
	s.Port = int(m.kSharePort.Value())
	s.TorMode = []string{"off", "personal", "public"}[m.kTorMode.CurrentIndex()]
	s.TorExecutable = strings.TrimSpace(m.kTorExe.Text())
	s.CertFile = strings.TrimSpace(m.kCert.Text())
	s.KeyFile = strings.TrimSpace(m.kKey.Text())
	if s.TorMode != "off" {
		s.TorSHA256, e = node.FileHash(s.TorExecutable)
		if e != nil {
			m.walletError(e)
			return
		}
	}
	m.chainAction("Starting Kaspa sharing", "Kaspa sharing started. Tor may still be connecting; refresh the page for its address.", func() error { return startKaspaSharing(m.root, s) })
}
func (m *manager) refreshKaspaSharing() {
	if m.kShareState == nil || m.kShareDetails == nil {
		return
	}
	active := serviceActive(kaspaAccessRoot(m.root), "sharing")
	s, e := readKaspaSharing(m.root)
	if e != nil {
		m.kShareState.SetText(e.Error())
		return
	}
	state := "Kaspa HTTPS / Tor stopped"
	if active {
		state = "Kaspa sharing running"
		if r, e := serviceCommand(kaspaAccessRoot(m.root), "sharing", "status"); e == nil && s.TorMode != "off" && !r.TorReady {
			state += " • Tor connecting"
		}
	}
	m.kShareState.SetText(state)
	c, _ := chains.Read(m.root)
	details := fmt.Sprintf("Peer access: %t • TCP %d\r\n", c.PublicP2P, c.P2P)
	if s.HTTPS {
		base := "wss://" + s.Host + fmt.Sprintf(":%d", s.Port)
		details += "Borsh: " + base + "/borsh\r\nJSON: " + base + "/json\r\n"
	}
	if h := s.OnionHost(kaspaAccessRoot(m.root)); h != "" {
		details += "Tor Borsh: ws://" + h + "/borsh\r\nTor JSON: ws://" + h + "/json\r\n"
	}
	details += "Saved settings; external reachability has not been tested."
	if m.kShareDetails.Text() != details {
		m.kShareDetails.SetText(details)
	}
	m.kShareStart.SetEnabled(!m.busy && !active && serviceActive(m.root, "kaspa"))
	m.kShareStop.SetVisible(active)
	m.kShareStop.SetEnabled(!m.busy && active)
	m.kPeerApply.SetEnabled(!m.busy && !serviceActive(m.root, "kaspa"))
	m.kPeer.SetEnabled(!m.busy && !serviceActive(m.root, "kaspa"))
	for _, w := range []walk.Widget{m.kHTTPS, m.kPublic, m.kHost, m.kSharePort, m.kCert, m.kKey, m.kTorMode, m.kTorExe} {
		w.SetEnabled(!m.busy && !active)
	}
}
