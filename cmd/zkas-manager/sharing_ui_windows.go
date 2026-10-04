package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"zkas-node-manager/internal/node"
)

type sharingUI struct {
	shared                                      node.SharingConfig
	shareHTTPS, sharePublic, peerPublic         *walk.CheckBox
	shareHost, shareTorExe, shareCert, shareKey *walk.LineEdit
	sharePort                                   *walk.NumberEdit
	shareTorMode                                *walk.ComboBox
	shareDetails                                *walk.TextEdit
	shareState                                  *walk.Label
	shareStart, shareStop, peerApply            *walk.PushButton
}

func (m *manager) sharingTab() TabPage {
	torIndex := 0
	if m.shared.TorMode == "personal" {
		torIndex = 1
	}
	if m.shared.TorMode == "public" {
		torIndex = 2
	}
	pick := func(field **walk.LineEdit, filter string) {
		d := walk.FileDialog{Title: "Choose file", Filter: filter}
		if ok, e := d.ShowOpen(m.window); e == nil && ok {
			(*field).SetText(d.FilePath)
		}
	}
	return TabPage{Title: "Sharing", Layout: VBox{MarginsZero: true}, Children: []Widget{
		ScrollView{Layout: VBox{Spacing: 18}, Children: []Widget{
			Label{Text: "What do you want to share?", Font: Font{PointSize: 16, Bold: true}},
			TextLabel{Text: "Choose any of the three options below. They are separate choices, not steps. Leave an option off if you do not need it.", MinSize: Size{Width: 240}},
			Label{AssignTo: &m.shareState, Text: "Sharing is off until you apply it."},
			sharingPanel("OTHER NODES", "Help other nodes sync", "For other node operators. This does not connect wallets or miners.", walk.RGB(29, 78, 140), walk.RGB(236, 244, 255), []Widget{
				CheckBox{AssignTo: &m.peerPublic, Text: "Allow other nodes to connect to my node", Checked: m.cfg.PublicP2P},
				PushButton{AssignTo: &m.peerApply, Text: "Save node-sharing choice", OnClicked: m.applyPeers},
				TextLabel{MinSize: Size{Width: 240}, Text: "To enable: stop services in Overview, check the box, save this choice, then start services again. In your router, forward TCP port 16811 to this PC.\nAddress to share: your public IP or domain followed by :16811. Example: node.example.com:16811."},
			}),
			sharingPanel("WALLETS · INTERNET", "Connect wallets using HTTPS", "For wallets and apps outside your home network. Requires a public IP or domain and router port forwarding.", walk.RGB(18, 104, 75), walk.RGB(235, 249, 241), []Widget{
				CheckBox{AssignTo: &m.shareHTTPS, Text: "Enable HTTPS access", Checked: m.shared.HTTPS},
				Label{Text: "Your public IP or domain name (example: node.example.com)"},
				LineEdit{AssignTo: &m.shareHost, Text: m.shared.Host},
				Composite{Layout: HBox{}, Children: []Widget{Label{Text: "HTTPS port"}, NumberEdit{AssignTo: &m.sharePort, MinValue: 1024, MaxValue: 65535, Decimals: 0, Value: float64(m.shared.Port)}}},
				CheckBox{AssignTo: &m.sharePublic, Text: "Public access: let other people use their wallets here", Checked: m.shared.PublicAPI},
				TextLabel{MinSize: Size{Width: 240}, Text: "Private: leave public access unchecked; your apps need an access token. Public: check it to let other people use the node. Compatible wallets can send and receive; spending keys stay on their devices.\nForward the HTTPS port above in your router to this PC. Apply this choice in the wallet-sharing controls below."},
				GroupBox{Title: "Certificate settings — optional for private use", Layout: VBox{Spacing: 6}, Children: []Widget{
					Composite{Layout: HBox{}, Children: []Widget{LineEdit{AssignTo: &m.shareCert, Text: m.shared.CertFile}, PushButton{Text: "Certificate…", OnClicked: func() { pick(&m.shareCert, "PEM (*.pem)|*.pem|All files (*.*)|*.*") }}}},
					Composite{Layout: HBox{}, Children: []Widget{LineEdit{AssignTo: &m.shareKey, Text: m.shared.KeyFile}, PushButton{Text: "Private key…", OnClicked: func() { pick(&m.shareKey, "PEM / key|*.pem;*.key|All files (*.*)|*.*") }}}},
					TextLabel{MinSize: Size{Width: 240}, Text: "Blank = generate a self-signed certificate. Clients must explicitly trust it. For public use with ordinary clients, supply a CA-issued domain certificate; renew it and restart sharing before expiry."},
				}},
			}),
			sharingPanel("WALLETS · TOR", "Connect wallets using an onion address", "An alternative to HTTPS. Tor creates the address for you; no domain or router port forwarding is needed.", walk.RGB(108, 54, 150), walk.RGB(246, 239, 252), []Widget{
				Label{Text: "Who can connect?", Font: Font{Bold: true}},
				ComboBox{AssignTo: &m.shareTorMode, Model: []string{"Off", "Personal onion — client key required", "Public onion — share the URL"}, CurrentIndex: torIndex},
				TextLabel{MinSize: Size{Width: 240}, Text: "Download and extract the Windows x64 Tor Expert Bundle using the button below, then select its tor.exe file. Keep the supporting DLL files beside it."},
				Composite{Layout: HBox{}, Children: []Widget{LineEdit{AssignTo: &m.shareTorExe, Text: m.shared.TorExecutable}, PushButton{Text: "Select tor.exe…", OnClicked: func() { pick(&m.shareTorExe, "Tor (tor.exe)|tor.exe") }}}},
				PushButton{Text: "Open official Tor download page", OnClicked: func() { openExternal("https://www.torproject.org/download/tor/") }},
				TextLabel{MinSize: Size{Width: 240}, Text: "Personal = only people with your client key. Public = anyone with the onion URL. Wallet apps must support Tor. Apply this choice using the wallet-sharing controls below.\nThis protects incoming wallet connections only; the node still makes ordinary internet connections to other nodes."},
			}),
			sharingPanel("APPLY / CONNECT", "Wallet-sharing controls", "These controls apply to HTTPS and Tor above. Node sharing has its own Save button in the blue panel.", walk.RGB(55, 65, 81), walk.RGB(242, 244, 247), []Widget{
				Label{Text: "After choosing HTTPS and/or Tor:", Font: Font{Bold: true}},
				Composite{Layout: VBox{MarginsZero: true}, Children: []Widget{
					PushButton{AssignTo: &m.shareStart, Text: "Apply choices / start wallet sharing", OnClicked: m.startSharingClicked},
					PushButton{AssignTo: &m.shareStop, Text: "Stop HTTPS / Tor access", OnClicked: func() { m.action(func() error { return stopOne(m.root, "sharing") }) }},
				}},
				Label{Text: "Your saved connection details", Font: Font{PointSize: 11, Bold: true}},
				TextLabel{Text: "After starting, copy the address for your wallet app. These are saved settings; availability from the internet is not tested.", MinSize: Size{Width: 240}},
				TextEdit{AssignTo: &m.shareDetails, ReadOnly: true, VScroll: true, MinSize: Size{Height: 150}, Text: m.shared.ConnectionDetails(m.root)},
				Composite{Layout: HBox{}, Children: []Widget{
					PushButton{Text: "Copy connection details", OnClicked: func() { m.copyText("Connection details", m.sharingDetails()) }},
					PushButton{Text: "Copy HTTPS access token", OnClicked: func() {
						if m.shared.Token == "" {
							walk.MsgBox(m.window, "Access token", "Start personal HTTPS sharing first.", walk.MsgBoxIconInformation)
							return
						}
						m.copyText("Private access token", m.shared.Token)
					}},
				}},
				PushButton{Text: "Copy personal onion client credentials", OnClicked: func() {
					s, e := m.shared.OnionClientCredentials(m.root)
					if e != nil {
						walk.MsgBox(m.window, "Onion credentials", e.Error(), walk.MsgBoxIconWarning)
						return
					}
					m.copyText("Private onion credentials", s)
				}},
				TextLabel{MinSize: Size{Width: 240}, Text: "Personal onion credentials go in a .auth_private file in the client's ClientOnionAuthDir. Treat them as a password. Compatible Tor clients may accept the final key directly."},
				PushButton{Text: "Open sharing files (keep keys private)", OnClicked: func() { m.open(filepath.Join(m.root, "sharing")) }},
				PushButton{Text: "Show setup / URL guide", OnClicked: func() { walk.MsgBox(m.window, "Public addresses and URLs", sharingGuide, walk.MsgBoxIconInformation) }},
				TextLabel{MinSize: Size{Width: 240}, Text: "Closing the manager leaves sharing running. Stop API sharing closes HTTPS/onion access but does not change peer access. Sharing does not auto-start after a reboot."},
			}),
		}},
	}}
}

// Each option has its own border, tinted surface, padded content and heading.
// Text and borders identify sections even without color perception.
func sharingPanel(tag, title, description string, accent, surface walk.Color, children []Widget) Widget {
	body := []Widget{
		Label{Text: tag, Font: Font{PointSize: 9, Bold: true}, TextColor: accent},
		TextLabel{Text: title, MinSize: Size{Width: 240}, Font: Font{PointSize: 14, Bold: true}, TextColor: accent},
		TextLabel{Text: description, MinSize: Size{Width: 240}, Font: Font{PointSize: 10}},
		HSeparator{},
	}
	body = append(body, children...)
	return Composite{Border: true, Background: SolidColorBrush{Color: surface}, Layout: VBox{Margins: Margins{Left: 16, Top: 14, Right: 16, Bottom: 16}, Spacing: 10}, Children: body}
}

const sharingGuide = `PUBLIC PEERS
Enable peer access with services stopped, then start services. Reserve this PC's LAN IP in your router and forward external TCP 16811 to the same port on the PC. Share PUBLIC_IP:16811 or YOUR_DOMAIN:16811. This manager uses 16811 explicitly; peers must specify it.

HTTPS WALLET API
Enter your public IP/domain and start sharing. Forward its TCP port (default 8443) to this PC. Share https://HOST:8443. For a domain, create a DNS A record pointing to your public IPv4 (use dynamic DNS if it changes). Do not publish an AAAA record unless you separately support IPv6.
A domain does not open ports. If your ISP uses CGNAT, ask for a public IP or use onion access. Check from a different internet connection; this manager cannot certify external reachability.

CERTIFICATES
A generated self-signed certificate requires deliberate client trust, using its displayed SHA-256 fingerprint. Do not disable TLS verification globally. A CA-issued certificate matching your domain works with standard clients; choose its full-chain PEM and private key here. This version does not obtain or renew CA certificates.

ONION
Download/extract the official Tor Expert Bundle and choose tor.exe. Select personal or public, start sharing, and wait for Tor bootstrap 100% in the Tor log. Copy the generated http://...onion URL. It works only through Tor. A generated hostname alone does not prove reachability.
Personal mode requires the copied client credentials in a .auth_private file under the client's ClientOnionAuthDir. Public mode needs no client key. Modes have different addresses; keys stay on this PC between launches.

APIs
Remote sharing exposes wallet REST only. gRPC, JSON wRPC, mining metrics, and seed/custodial/admin endpoints are not exposed. Applications retain their own setting names and X-Wallet-Token. For personal HTTPS also send Authorization: Bearer YOUR_ACCESS_TOKEN. Browser CORS access is disabled.`

func openExternal(url string) {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if cmd.Start() == nil {
		go cmd.Wait()
	}
}
func (m *manager) readSharingSettings() (node.SharingConfig, error) {
	s := m.shared
	s.HTTPS = m.shareHTTPS.Checked()
	s.PublicAPI = m.sharePublic.Checked()
	s.Host = strings.TrimSpace(m.shareHost.Text())
	s.Port = int(m.sharePort.Value())
	s.CertFile = strings.TrimSpace(m.shareCert.Text())
	s.KeyFile = strings.TrimSpace(m.shareKey.Text())
	s.TorExecutable = strings.TrimSpace(m.shareTorExe.Text())
	s.TorMode = []string{"off", "personal", "public"}[m.shareTorMode.CurrentIndex()]
	if s.TorMode != "off" {
		hash, e := node.FileHash(s.TorExecutable)
		if e != nil {
			return s, e
		}
		s.TorSHA256 = hash
	}
	return s, s.Validate(m.cfg)
}
func (m *manager) startSharingClicked() {
	s, e := m.readSharingSettings()
	if e != nil {
		walk.MsgBox(m.window, "Check sharing setup", e.Error(), walk.MsgBoxIconWarning)
		return
	}
	c := m.cfg
	summary := "Start the selected API sharing services? Your node and wallet stay running.\n\n"
	if s.HTTPS {
		summary += s.URL() + "\nWindows will request firewall approval. Router forwarding is still needed.\n"
		if s.PublicAPI {
			summary += "Other people can use compatible wallets to receive and send through this API. Their spending keys stay on their devices; scans and proofs use this PC's resources.\n"
		} else {
			summary += "HTTPS clients must provide your access token.\n"
		}
		if s.CertFile == "" {
			summary += "A self-signed certificate will be generated; clients must explicitly trust it.\n"
		}
	}
	if s.TorMode != "off" {
		summary += "Tor mode: " + s.TorMode + ". Onion access does not hide node peer traffic.\n"
	}
	if walk.MsgBox(m.window, "Start sharing", summary, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	m.action(func() error {
		if serviceActive(m.root, "sharing") {
			return fmt.Errorf("stop API sharing before applying changes")
		}
		host, e := sharingHostPath(m.root)
		if e != nil {
			return e
		}
		s.HostExecutable = host
		if s.Token == "" {
			s.Token, e = node.NewSharingToken()
			if e != nil {
				return e
			}
		}
		os.MkdirAll(filepath.Join(m.root, "logs"), 0700)
		os.Remove(serviceErrorFile(m.root, "sharing"))
		e = startSharing(m.root, c, s)
		if saved, err := node.ReadSharing(m.root); err == nil {
			m.onUI(func() { m.shared = saved })
		}
		return e
	})
}
func (m *manager) applyPeers() {
	if hostActive(m.root) {
		walk.MsgBox(m.window, "Stop services first", "Stop services from Overview before changing peer access. Opening this version does not change your current listeners.", walk.MsgBoxIconInformation)
		return
	}
	c := m.cfg
	c.PublicP2P = m.peerPublic.Checked()
	peerHost := strings.TrimSpace(m.shareHost.Text())
	if c.Version == "" {
		walk.MsgBox(m.window, "Install node first", "Install the node before applying public peer access.", walk.MsgBoxIconInformation)
		return
	}
	note := "Disable incoming public peer connections?"
	if c.PublicP2P {
		note = "Enable incoming public peer connections on TCP 16811?\n\nWindows will request firewall approval. Forward TCP 16811 in your router. Your wallet APIs stay local unless separately shared."
	}
	if walk.MsgBox(m.window, "Peer access", note, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	m.action(func() error {
		if hostActive(m.root) {
			return fmt.Errorf("services are still running")
		}
		if e := ensureInstalled(m.root); e != nil {
			return e
		}
		s, e := node.ReadSharing(m.root)
		if e != nil {
			return e
		}
		s.Host = peerHost
		s.HostExecutable, e = sharingHostPath(m.root)
		if e != nil {
			return e
		}
		if e = node.SaveSharing(m.root, s); e != nil {
			return e
		}
		if e = node.Save(m.root, c); e != nil {
			return e
		}
		m.onUI(func() { m.cfg = c; m.shared = s })
		return applySharingFirewall(s)
	})
}
func (m *manager) sharingDetails() string {
	out := m.shared.ConnectionDetails(m.root)
	if m.cfg.PublicP2P {
		h := m.shared.Host
		if h == "" {
			h = "YOUR_PUBLIC_IP"
		}
		out = "Peer listener enabled: " + h + ":16811 (requires running node + router forwarding)\r\n" + out
	}
	if m.shared.HTTPS {
		if fp, e := m.shared.CertificateFingerprint(m.root); e == nil {
			out += "\r\nTLS certificate SHA-256: " + fp
		}
	}
	return out
}
func (m *manager) refreshSharing() {
	if m.shareState == nil {
		return
	}
	active := serviceActive(m.root, "sharing")
	state := "API sharing stopped"
	if active {
		state = "API sharing running (external reachability unverified)"
		if m.shared.TorMode != "off" {
			r, e := serviceCommand(m.root, "sharing", "status")
			if e != nil {
				state = "Sharing host is not responding"
			} else if r.TorReady {
				state += " · Tor bootstrapped"
			} else {
				state += " · Tor connecting (see Tor log)"
			}
		}
	} else if b, e := os.ReadFile(serviceErrorFile(m.root, "sharing")); e == nil {
		state = "API sharing error: " + string(b)
	}
	m.shareState.SetText(state)
	m.shareDetails.SetText(m.sharingDetails())
	for _, w := range []walk.Widget{m.shareHTTPS, m.shareHost, m.sharePort, m.sharePublic, m.shareTorMode, m.shareTorExe, m.shareCert, m.shareKey} {
		w.SetEnabled(!m.busy && !active)
	}
	m.shareStart.SetEnabled(!m.busy && !active)
	m.shareStop.SetEnabled(!m.busy && active)
	m.peerApply.SetEnabled(!m.busy && !hostActive(m.root))
	m.peerPublic.SetEnabled(!m.busy && !hostActive(m.root))
}
