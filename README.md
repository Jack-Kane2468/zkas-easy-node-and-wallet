# ZKas Node Manager

A Windows desktop manager for running a ZKas node, connecting applications, managing local wallets, and setting up solo mining or optional public access.

**Community preview · Windows x64 · unsigned · independently maintained.** This project is not an official ZKas, Zcash or Kaspa product. Read [validation and limitations](VALIDATION.md) before relying on wallet or public-hosting features.

## Download and run

1. Open this repository's **Releases** page and download the **Windows-x64 ZIP** attached to a release. GitHub's **Code → Download ZIP** is source code, not the runnable application.
2. Extract the **whole ZIP** to a folder. Keep `wallet-runtime` next to `ZKasNodeManager.exe`.
3. Open `ZKasNodeManager.exe`. Click **Overview → Install / setup** to open Settings, choose **Install / repair components**, then use **Overview → Start services** if needed. Initial blockchain synchronization takes time.

No Go, Rust or separate Node.js installation is needed for a release ZIP. The node and optional mining bridge are downloaded during installation; an internet connection is required. The manager does not mine automatically.

Windows may warn because the EXE is unsigned. Verify the download source and checksum; do not disable antivirus or global Windows protection. Firewall changes request Windows administrator approval; ordinary manager use runs as your user.

## What it does

| Tab | Purpose |
| --- | --- |
| Overview | Open installation/setup, start/stop services, see sync state, copy API addresses and check manager or node updates. |
| Wallet | Create/import multiple wallets, receive/send, back up keys, view the selected wallet’s transaction history, and use independent FVK/OVK tools. |
| Mining | Install or uninstall the bridge, connect miners through Stratum TCP 5555, and view bridge-reported workers/hashrate. |
| Sharing | Configure incoming peers, HTTPS wallet API access, or personal/public Tor onion access. |
| Settings | Node storage and ports, startup options, installation and uninstall. |
| Logs | Inspect node, wallet backend, bridge and Tor logs locally. |

Closing the window leaves services running. These are background processes owned by your Windows user, not Windows Service Control Manager services. Autostart runs at sign-in.

The built-in wallet stores secrets in a password-encrypted local vault. Keep your own recovery backup; uninstall retains user data. Never share seeds, spending keys, vault files, API tokens, or viewing keys in issues. Viewing keys reveal private financial history even though they cannot spend funds.

## Updating

Use **Overview → Check manager updates** to check for newer application releases. Preview releases require **Include preview releases** to be enabled. The manager displays release notes before confirmation, verifies the package and restarts its window. Existing data and running services are retained; the wallet vault locks.

**Check node updates** updates the upstream node separately. See [Manager updates](MANAGER-UPDATES.md) for details.

## Local connections and public access

Local-only use is the default. Copy the API addresses shown in Overview instead of guessing ports. Sharing is optional; a working local node does not need router forwarding. Public API hosting consumes resources and exposes the operator's network; the gateway's concurrency/body limits are not a full abuse-prevention system. See [Sharing](SHARING.md).

## Guides

- [Wallets](WALLETS.md) — account numbers, backup, receive/send and exports.
- [Wallet tools](WALLET-TOOLS.md) — independent FVK/OVK scans and their limits.
- [Mining](MINING.md) — bridge, miner setup and uninstall.
- [Sharing](SHARING.md) — peers, HTTPS, onion addresses and firewall setup.
- [Application connections](CONNECTIONS.md) — local APIs for bots and tools.
- [Build instructions](BUILDING.md) and [contributing](CONTRIBUTING.md).
- [Security](SECURITY.md), [validation](VALIDATION.md), [changelog](CHANGELOG.md).

## License and upstream components

Manager code is MIT licensed. The viewing helper is ISC licensed. Other bundled code keeps its own licenses; see [third-party notices](THIRD-PARTY-NOTICES.md) and [runtime provenance](wallet-runtime/PROVENANCE.md).

Node/bridge upstream: https://github.com/firecash/zkas-rusty

Signer upstream: https://github.com/firecash/zkas-wallet
