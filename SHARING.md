# Public and personal connections

The Sharing tab groups controls in separate bordered panels: blue for other nodes, green for HTTPS wallets, purple for Tor wallets, and gray for wallet-sharing actions and saved addresses. Headings and borders also distinguish them without relying on color.

These are independent options, not steps: public node peers, HTTPS wallet access, and Tor onion wallet access. You may use any combination. Nothing becomes public merely by opening the manager. Apply/start each option explicitly.

Wallet clients can receive and send through the shared API: they keep spending keys on their own devices. The server handles viewing/scanning, prepares transactions and broadcasts signed submissions. The built-in Wallet tab is your personal client and works without public sharing.

## Help other nodes sync: public peers

1. Stop services from Overview.
2. Check Public peer listener and Apply peer setting. Accept the Windows firewall prompt.
3. Start services from Overview.
4. Reserve this PC's LAN IPv4 in your router, then forward TCP **16811** to port 16811 on that PC.
5. Give peers `YOUR_PUBLIC_IP:16811` or `YOUR_DOMAIN:16811`.

The manager explicitly uses 16811, even if other ZKas tools have another default P2P port. Include the port. This is a raw node peer address, not a website URL. The firewall allows this executable/port on all Windows network profiles. UPnP remains disabled; no router settings change automatically. Disable public peers using the same stopped-services Apply flow to remove that rule and return the node listener to loopback.

Enabling public peers does not share your wallet API. Node RPC listeners remain loopback.

## HTTPS wallet API

1. Keep node/wallet services running.
2. Enable HTTPS in Sharing. Enter an IP or domain without a scheme/path/port. Default external port: **8443**.
3. For personal access leave Public HTTPS API unchecked; clients need the generated access token. For community use check it to allow watch-only registrations without a gateway token. Each wallet still needs its own wallet token.
4. Choose a certificate full-chain PEM and private-key PEM if you have a CA-issued certificate matching your hostname. Leaving both blank generates a self-signed certificate valid for one year.
5. Apply / start API sharing and accept Windows firewall approval.
6. Forward your selected TCP port in the router to the same port on this PC. Copy the resulting `https://HOST:PORT` URL.

Self-signed TLS encrypts traffic but is not automatically trusted. Clients must trust the specific certificate and verify its displayed SHA-256 fingerprint through a trusted channel. Do not disable certificate verification globally. Normal public clients generally need a CA-issued domain certificate. This preview does not obtain or renew CA certificates; renew externally and restart sharing to load the replacement.

For a domain, create a DNS A record pointing to your public IPv4. Dynamic DNS can help if your ISP changes that address. DNS does not open ports or bypass CGNAT. This gateway listens on IPv4; do not publish an AAAA record expecting it to work automatically. With CGNAT ask the ISP for an inbound-capable public IP or use onion access.

Test from a separate internet connection. A running local gateway does not prove your DNS/router/firewall/ISP path is reachable. The manager explicitly displays external reachability as unverified. No credentials are included in Copy connection details; Copy HTTPS access token is a separate action.

The gateway binds the chosen HTTPS port, forwards only documented watch-only wallet endpoints, blocks custodial/admin/unknown routes, limits request bodies to 4 MiB and concurrent forwarded operations to 16. It is not comprehensive DDoS protection or per-user resource accounting. Public registrations/scans/proofs consume disk, CPU and memory. It does not expose node gRPC, wRPC or mining administration.

## Tor onion wallet API

1. Download and extract the **Windows x64 Tor Expert Bundle** from https://www.torproject.org/download/tor/ . Use the official signature verification instructions for that download. Keep the extracted DLLs/supporting files; do not move only tor.exe.
2. Select the extracted **tor.exe** in Sharing.
3. Choose **Personal onion** or **Public onion**, then Apply / start API sharing. HTTPS may be on or off independently.
4. Wait for Tor bootstrap. The Sharing status reports Tor connecting/bootstrapped; Logs > Tor log has details. The generated hostname appears in connection details.
5. Copy `http://56-characters.onion`. It works through a Tor-capable client, not ordinary DNS. No bought domain, public IP, inbound Windows firewall rule or router forwarding is needed for onion access.

The local HTTP target is loopback port 18502; traffic over the onion circuit is protected by Tor. A hostname file may exist before bootstrap completes, and bootstrap alone is not an end-to-end reachability test.

### Personal onion authorization

Personal onion mode requires a Tor X25519 client authorization key, generated automatically and retained. Copy personal onion client credentials; the format is:

`ONION_HOST_WITHOUT_SUFFIX:descriptor:x25519:PRIVATE_KEY_BASE32`

On the connecting device, save that one line in a file ending `.auth_private` under its Tor `ClientOnionAuthDir`, then restart/reload the client as its documentation requires. Tor Browser can accept the final private-key component in its authorization prompt. The wallet/application itself still needs Tor routing and normal wallet API support. Many applications do not implement onion authorization directly; configure their Tor client accordingly. Do not post the credentials publicly.

Public onion mode needs no client authorization key and can be shared freely. The manager uses distinct directories/identities for personal and public onions, so switching mode does not publish your personal address. Identities survive app updates and restarts. Stop sharing before changing mode.

### What Tor does and does not cover

This feature hosts an onion endpoint for wallet REST access. It does **not** proxy the node's outgoing P2P connections, conceal ordinary peer traffic or implement onion-only node peering. The inspected Windows node release did not provide verified supported Tor P2P flags. Do not assume the computer is anonymous because it has an onion API.

The manager starts a separate Tor process with its own data directory, no SOCKS listener and no control listener. It does not modify an existing Tor Browser profile. Tor must be installed/updated separately; the chosen executable is hashed when applied and checked again before launch. Private sharing material has a current-user/SYSTEM Windows ACL.

## Stop/restart and data

Stop API sharing closes the HTTPS/onion listeners and its Tor process. It does not stop the node, wallet or miners and does not change public peer access. Windows API firewall rules can remain while stopped, but no gateway listens. Disabling HTTPS and applying an onion-only configuration removes the manager's HTTPS rule. Uninstall removes both managed public rules. Closing the window leaves sharing running; reboot does not automatically restart sharing.

Keep `%LOCALAPPDATA%\ZKasNodeManager\sharing` private. It contains access tokens, TLS keys and onion keys. Back up privately if stable identity matters. Deleting an identity changes its URL and access credentials. This preview has no one-click key revocation/rotation UI: stop sharing before managing keys. Never distribute your installed data folder as the installer package.

References:
- https://community.torproject.org/onion-services/setup/
- https://community.torproject.org/onion-services/advanced/client-auth/
- https://github.com/firecash/zkas-rusty/tree/zkas-v1.0.9
