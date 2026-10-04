# Solo mining and live statistics

1. Open Mining and click **Install mining bridge**. The bridge is the extra program that connects ASIC / Stratum miners to the node. You can install while the node is syncing. This downloads the official bridge matching your installed node; it does not open a port or start mining.
2. Wait for node consensus sync, select the PC address on your miners' network, and enter the public payout address and worker name you want to copy.
3. Click **Start mining (open LAN port 5555)** to start a separate bridge connected to the existing node.
4. Accept the Windows firewall prompt: TCP **5555**, Private profile, LocalSubnet, bridge executable only.
5. Configure your compatible kHeavyHash miner:

- Pool: `stratum+tcp://PC_LAN_IP:5555`
- Username: `YOUR_ZKAS_ADDRESS.worker1`
- Password: `x`

This is solo mining. Rewards depend on finding valid blocks. It is not shared-reward pool accounting. An address entered in this manager helps produce the copied settings; configure that address in each actual miner. The bridge does not enforce this field as a universal payout override.

## Live view

The manager polls the bridge's local `/api/stats` endpoint on **127.0.0.1:18888**. It shows:

- Established IPv4 TCP sessions to port 5555, from the Windows TCP table. A session does not prove successful Stratum authorization or accepted shares.
- Workers with recent share activity, names, wallet addresses and online/idle status reported by the bridge. The bridge retains active workers for up to five minutes after activity; this differs from current socket count.
- Estimated GH/s from reported worker share statistics, including total and per-worker estimates. This is not the miner's own hardware measurement and can fluctuate.
- Valid shares, stale shares, invalid shares, per-worker reported blocks, total reported blocks and recent block hashes.
- Bridge uptime and the local time of the latest successful poll.

Counters are bridge-process scoped, not lifetime accounting. Reported blocks are not a guarantee of confirmed spendable rewards. If statistics fail, the manager displays unavailable rather than presenting zeros or old data as live. The dashboard uses the matching v1.0.9 bridge schema; other upstream versions can change that interface.

If mining was already running under v0.3, click **Stop mining**, then **Start mining (open LAN port 5555)** once. Only the mining bridge restarts, enabling its loopback dashboard; node and wallet services can keep running. Port 18888 must be free.

Close the manager to leave mining running. Stop mining affects only the bridge. Close LAN port also removes its firewall rule. No mining autostart is installed. A node update can require a matching bridge update before the next mining start.

If miners cannot connect, verify the PC is on a Private Windows network, both devices are on the same subnet, the selected PC IP is correct and the bridge is running. VPNs, VLANs, guest Wi-Fi isolation and an occupied port 5555 can interfere. This setup does not open the Stratum port to the public internet.

Upstream statistics source: https://github.com/firecash/zkas-rusty/blob/zkas-v1.0.9/bridge/src/prom.rs

## Uninstall
Use **Uninstall mining bridge…** to stop miners, remove bridge files and remove its Windows TCP 5555 firewall rule. Node, wallet and sharing services remain running. Payout/worker preferences remain for reinstalling. The main Settings → Uninstall includes this same removal. If Windows firewall approval is declined, removal stops and reports the error; retry when ready.
