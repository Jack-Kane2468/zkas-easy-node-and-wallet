# Kaspa node, wallet and mining

## Node and settings

Open **Overview → Kaspa → Install Kaspa + start syncing**. The manager downloads the pinned official Windows release, verifies its SHA-256 and starts a separate mainnet node. Setup disappears once installed; Start and Stop reflect its running state. The status panel shows node readiness and its latest synchronization message.

**Settings → ZKas** contains ZKas configuration. **Settings → Kaspa** contains node purpose, blockchain data folder, API/peer ports, cache memory scale, automatic startup, repair and uninstall. Stop Kaspa before changing its memory scale. The default 0.3 reduces caches; it is not a hard memory limit. Both nodes and wallet scans can exceed an 8 GB PC's memory.

Kaspa defaults to wallet/application mode: pruned with the UTXO index enabled. Basic/mining mode omits the index. Archive mode retains blocks instead of pruning and includes the wallet index; changing to it requires an empty data folder and does not recreate history already pruned elsewhere. RPC listeners remain local; peer sharing is optional. UPnP and unsafe RPC remain disabled. Kaspa data lives in the manager's `kaspa-data` folder, separate from ZKas. Closing the manager leaves nodes and mining running; the built-in wallet locks. ZKas and Kaspa have separate sign-in startup choices. Mining and sharing start manually.

## Graphical Kaspa wallet

Open **Wallet → Kaspa** and choose **Unlock / open wallet**. The separate Kaspa vault uses one password for its wallets. Create a wallet or import its phrase/private key, select it, then connect to your local node.

- **Create wallet** generates a 24-word recovery phrase. Save it before confirming.
- **Import phrase / key** accepts a BIP39 phrase or 64-character private key. Account number 0 is the normal default; other numbers derive different accounts. The optional BIP39 passphrase must match the original recovery secret.
- **Unlock / open wallet** switches to the selected wallet. **Lock vault** closes the wallet process; node services keep running.
- **Connect / refresh balance** uses the configured local Kaspa node. Sending requires a synchronized wallet balance. Account totals cover its receiving and change addresses.
- **Receive** shows the current address and its real SDK derivation index. **Generate next address** advances the SDK receiving index. A single private-key account cannot generate additional addresses.
- **Addresses and UTXOs** lists receive and change branches separately with their indexes, addresses, unspent amounts and output counts. Select a row for transaction outpoints, values, block DAA scores and mining-reward flags. Copy the selected address directly from this panel.
- Address pages contain up to 50 rows from the SDK's known index ranges. UTXO totals are point-in-time snapshots from the node; use **Refresh address balances** to update them. Unknown/offline values are not shown as zero. Unspent amounts can include immature rewards. Zero current UTXOs does not mean an address was never used.
- **Export selected receiving key** reveals the private key for a selected Receive row after vault-password confirmation. The key is checked against that address. If the vault does not contain the recovery phrase/key, enter it once; a matching recovery secret is then saved encrypted in the vault. The exported address key is not a backup of the entire wallet.
- **Send KAS** reviews the amount, recipient and fee. Coin selection uses the account, not the highlighted address row. A local receipt is saved before submission; uncertain submissions are not automatically retried.
- **Show transaction history** displays wallet-recorded transactions and local send receipts. A pruned node cannot recover every previously spent transaction after a phrase import.
- **Back up vault** stores its wallets and encrypted SDK file snapshots in an encrypted JSON backup. **Restore vault** adds separate wallet copies without overwriting current wallets. Retain original recovery phrases/private keys too.
- **Change vault password** changes the current vault password; exported backups keep their original password. **Remove wallet** removes a wallet from the active list after confirmation and retains an encrypted recovery copy.

Kaspa addresses and transactions are transparent. A fresh address is not an anonymity guarantee. Derivation, wallet scanning and signing use the official Kaspa v2.1.0 SDK; the manager verifies generated addresses against that account's SDK descriptor. Secrets use private process pipes. SDK files live under `kaspa-wallets`, and the encrypted vault lives under `kaspa-vault`; normal uninstall preserves both.

## One Mining tab

**Mining → ZKas** contains the original ZKas bridge, including its saved payout address, worker, difficulty, install/uninstall controls and live statistics. It continues reading the original `mining.json`; existing installations do not need conversion or reinstallation.

**Mining → Kaspa** contains the official Kaspa-only bridge. **Mining → Merged** contains the Kaspa + ZKas bridge. There is no mode dropdown. The installation inventory includes the original ZKas bridge and the merged-capable bridge. The ZKas page uses the original bridge when installed, or the merged-capable bridge in ZKas-only mode otherwise. Each page shows its own start/stop, installation and uninstall state.

1. Start and sync the node(s) required by the selected mode.
2. Enter receiving addresses; never enter private keys or seed phrases in mining fields.
3. Install the selected bridge if needed, then start mining.
4. Paste the displayed `stratum+tcp://` server, username and password into compatible mining hardware/software.

All modes use TCP 5555, so stop the active bridge before starting another. For the original ZKas bridge, LAN startup allows Windows Private/local-subnet access. The Kaspa/merged page offers an explicit LAN checkbox; unchecked means this PC only. Router forwarding is unchanged.

For merged mining, username is the ZKas address and password is the Kaspa address. Saved operator addresses are the upstream fallback for missing miner addresses. Kaspa-only uses the Kaspa address as username and `x` as password. Single-chain modes do not require the other node.

The original ZKas statistics remain inside its mining subtab. The Kaspa/merged **Open live mining dashboard** button opens `http://127.0.0.1:18889`. Hashrate is estimated from shares; found blocks are not guaranteed confirmed rewards. This is solo mining, not a pool or a CPU/GPU miner.

## Developer connections

| API | Local endpoint |
| --- | --- |
| gRPC | `127.0.0.1:16110` |
| JSON wRPC | `ws://127.0.0.1:18110` |
| Borsh wRPC | the configured local Borsh endpoint (default `ws://127.0.0.1:17110`) |

Use a Kaspa SDK for queries, subscriptions and signed transaction submission. The node does not store wallet keys. Sharing → Kaspa offers peer access, HTTPS/WSS and personal/public Tor access. All are off by default. Other reserved ports: default P2P 16111, dashboard 18889, health 18081, metrics 18115 and Kaspa Tor gateway 18503. API and P2P ports can be changed in Settings; the wallet and mining bridge follow those settings.

## Versions and validation

Node/bridge releases are pinned to Kaspa v2.1.0 and solo-dual-mode v1.0.11. **Check node updates** updates ZKas; future manager releases can change the Kaspa pins. Repair/uninstall controls are under Settings → Kaspa. Main uninstall removes managed node/bridge programs while retaining databases and wallet files.

This preview is cross-built for Windows. Offline wallet integration and portable Go tests run during validation. Native Windows UI/firewall operation, live wallet payments and actual mining payouts have not been exercised in the build environment. See VALIDATION.md.


## Sharing Kaspa

**Sharing → Kaspa** has separate panels for peers, HTTPS/WSS and Tor. These are independent choices, not sequential steps. Stop Kaspa before changing peer access. Router forwarding is manual: forward the selected peer port for other nodes, or the HTTPS port for wallet/app access. Share peer addresses as `HOST:PORT`.

The secure wallet/app gateway defaults to port 8444, separate from ZKas's 8443. Its endpoints are `wss://HOST:PORT/borsh` and `wss://HOST:PORT/json`. Private HTTPS requires `Authorization: Bearer TOKEN` in the WebSocket upgrade; use an SDK that supports custom headers. Public mode needs no token. This exposes Kaspa's safe node RPC, including signed transaction submission, not a server-side wallet or seed API. gRPC stays local. The gateway limits concurrent connections to 64 per listener.

Self-signed certificates need explicit client trust. Public clients normally need a CA-issued certificate matching the domain. Certificates are not automatically obtained or renewed. Create a DNS A record for a domain and test from a different internet connection; a saved address is not proof of reachability. CGNAT may require a public IP from the ISP or Tor.

Personal and public onion services use separate identities stored under `kaspa-access`, also separate from ZKas. Select `tor.exe` from the official Tor Expert Bundle and retain its DLLs. Personal Tor uses client authorization; copy the credentials into the client's `.auth_private` file. Onion WebSocket URLs end in `/borsh` or `/json`. The client must support Tor. Kaspa's outbound peer connections are not routed through this incoming-access feature.

Stopping Kaspa stops its sharing and dependent mining. Stopping ZKas leaves Kaspa-only mining running; stopping Kaspa leaves ZKas-only mining running. Uninstall preserves blockchain and wallet data and removes managed firewall/startup rules. Main uninstall handles both chains.

## Processes and monitoring

Both projects name their node executable `kaspad.exe`. Running both chains therefore creates two independent node processes; one cannot serve both networks. The overview identifies each node separately.

Status monitoring reuses persistent gRPC streams instead of opening a new connection per poll. Initial connections, reconnects after a node restart, and explicit setup checks can still appear in logs. The Kaspa overview has text status like ZKas and no animated loading bar.
