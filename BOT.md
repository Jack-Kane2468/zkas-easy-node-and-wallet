# Telegram bot

The Bot tab runs your own Telegram bot. **Stop bot when manager closes** is checked by default. Uncheck it and save/start to keep the bot running in the system tray after closing the desktop window. The tray offers **Open node manager** and **Stop bot and quit manager**. Closing into the tray locks desktop wallets but keeps Telegram sessions active. This is a background process, not a Windows service; signing out, rebooting or terminating it stops the bot. It uses outbound HTTPS long polling. No public node, router forwarding or webhook server is required. Use a dedicated bot token.

## Setup

1. Create a bot with Telegram's verified **@BotFather** and paste its token into **Bot → Bot token**.
2. Leave **Owner only** checked. Enter your numeric Telegram user ID, or leave it blank to pair with the one-time command shown after starting. Approve the matching ID in the manager.
3. Choose the permissions and alerts you want, then **Save and start bot**.
4. Open a private chat with your bot and send `/menu`, or tap **Menu → Open manager menu**.

For additional users, stop the bot, uncheck Owner only, add their numeric user IDs, and restart. All allowed users receive the same selected permissions. Unknown users and group chats are ignored. Each alert recipient must first start a private conversation with the bot. Additional recipients are remembered after they use it.

Stop the bot before editing settings. Closing/updating the manager stops the bot; restart it in the Bot tab. Node and bridge services remain independent. Configuration, tokens, viewing keys and recipient IDs are encrypted for the current Windows user.

## Menus and optional commands

Button clicks update the existing bot message and buttons. Typed commands and payment inputs receive a fresh reply at the bottom of the chat; a long-running request may update that new progress reply with its result. If that message was deleted or cannot be edited, the bot sends a replacement. Alerts arrive as separate messages. Messages allow normal copying and forwarding.

| Permission | Menus / optional commands | Behavior |
| --- | --- | --- |
| Node / backend status | `/status` | Local service and node sync status. |
| Mining dashboard statistics | `/mining` | Bridge workers, shares, estimated hashrate and reported blocks. |
| Node controls | `/node start\|stop zkas\|kaspa` | Uses saved settings. Stopping a node also stops its dependent services. |
| Mining controls | `/bridge start zkas\|kaspa\|merged`, `/bridge stop` | Uses saved mining settings and checks required nodes. |
| Allow ZKas wallet access | `/wallets`, `/balance ID`, `/history ID`, `/receive ID`, `/sync ID` | Signed-in ZKas wallets in the independently unlocked Telegram vault session. Status requests can wake scans. |
| Allow Kaspa wallet access | `/kaspa_wallets`, `/kaspa_open WALLET_ID`, `/kaspa_account WALLET_ID ACCOUNT_ID`, `/kaspa` | Open any wallet in the unlocked Kaspa vault, select an account, view balance and receive address. Opening/selecting does not update or unlock the desktop. |
| Separate FVK wallets | `/fvks`, `/fvk_sync ID`, `/fvk_balance ID`, `/fvk_history ID` | Viewing keys configured locally; spending vault may stay locked. |
| Request a payment for local review | `/payment ID` | Opens the local ZKas send workflow, with approval on the PC. This existing permission does not enable remote spending. |
| Send from Telegram (unlocked wallets) | Send button; `/send zkas WALLET_ID` or `/send kaspa WALLET_ID ACCOUNT_ID` | Enter recipient and amount in chat; review and confirm there. Requires access to the selected chain too. Off by default. |

Wallet/account lists are paginated. Wallet menus supply IDs automatically. Addresses and transaction IDs have copy buttons. The current account is shown in the message, not repeated as a selection button. ZKas history shows up to ten latest returned entries. There are no Telegram commands for key export, updates, uninstall or sharing configuration.

## Sending from Telegram

Enable **Send from Telegram (unlocked wallets)** explicitly. Unlock the appropriate Telegram vault using **Unlock ZKas for bot** / **Unlock Kaspa for bot** in the Bot tab, or enable remote lock/unlock and unlock through Telegram. For ZKas, the wallet must be signed in and have spending material. For Kaspa, open the wallet/account using the bot's menu. Wait for wallet sync/readiness before sending.

The guided flow asks for recipient and amount, plus a maximum fee and optional encrypted memo for ZKas. Kaspa uses the SDK's estimated network fee. The review identifies the wallet/account, recipient, amount and fee. Tap **Confirm payment**, or reply `confirm` after the review appears. No long confirmation strings are required. Cancel abandons the draft.

Reviews expire after two minutes and are single-use. A queued typed confirmation from before delivery of the review cannot approve it. Wallet identity and vault access are checked again before signing; locking the vault or changing the selected Kaspa account requires a new review. A stale button cannot approve a different payment. Requests are never automatically retried after submission errors.

ZKas uses the existing official signer and amount/fee validation. Kaspa uses the existing SDK confirmation token and rejects an increased fee. The manager saves an attempt receipt before broadcasting. If submission outcome is unknown, inspect history before trying again. A broadcast receipt is not chain confirmation. Stopping the bot after submission begins does not cancel an already-broadcast transaction.

## Alerts

Three independent choices are available:

- **New blocks:** a rise in the active bridge's reported block counters. Merged mode distinguishes ZKas and Kaspa when both counters are available. Initial samples, bridge restarts and counter resets establish a new baseline; old blocks are not announced. These are bridge reports, not proof of accepted blocks or payouts.
- **Unexpected node stops:** a previously observed running ZKas/Kaspa node stops. Stops requested through this manager are suppressed. An exit status or recognized recent memory/disk/panic/port/access failure is included when available; arbitrary raw log lines are not forwarded. An external deliberate stop may look unexpected.
- **Sync/health:** loss of the node's synced flag, sustained RPC failure, or unchanged block/header/DAA progress for the configured 1–60 minutes (default 5). Recovery is reported. No progress can also mean an idle or isolated chain, so a stall alert is not a confirmed crash diagnosis.

Checks run approximately every 20 seconds using persistent node RPC streams. Brief incidents between checks can be missed. Alerts require this manager, bot and internet connection to remain running. Missed deliveries are not queued or replayed; the Bot tab reports delivery failure. Alerts do not restart services automatically.

## Privacy and access

Telegram bot chats are cloud chats, not end-to-end encrypted. Balances, addresses, payment details and entered memos pass through Telegram. Never enter recovery phrases, private keys, FVKs or vault passwords in chat. Viewing keys are added only in the local Bot tab. Secrets are not put into Telegram buttons or messages.

FVK-only access cannot spend. Removing a viewing key revokes its bot access after saving/restarting but does not delete its backend scan registration. Locking a Telegram vault revokes its Telegram session. Desktop lock state is independent; wallet deletion, password changes and sign-out are rechecked before spending. Enabling remote spending grants every allowed Telegram user authority to send from available unlocked wallets; keep the allowlist appropriate to that permission.

## Optional mining-down alerts

Enable the separate mining-down checkbox and set a minimum recently active worker count, an optional estimated hashrate floor in GH/s (0 disables that check), and a delay of 1–60 minutes. The default delay is three minutes. The same delay gives a newly started bridge time to receive shares.

Once a bridge has been observed running, an unexpected bridge stop, unavailable statistics API, too few active workers or a low estimated hashrate can trigger an alert after the condition persists. Unavailable statistics are explicitly unknown, not zero hashrate. Recovery is reported. Stops requested through the manager are suppressed. Recent-worker counts are share-based and can lag a miner disconnect; this is not instantaneous TCP monitoring.

## Optional daily summary

Choose an HH:MM delivery time in the computer's local timezone. The first summary is scheduled for the next occurrence of that time. It includes observed running percentages and current node status, average recently active workers and estimated hashrate, and newly observed bridge block counts. Enabled receive monitoring adds incoming-transaction counts for the permitted chains.

The summary describes the observation period and coverage. Time while the manager was closed or monitoring was interrupted is excluded; unavailable mining statistics are not treated as zero. Coverage is saved about once a minute. A restart retains the period and delivery date. After downtime, an overdue summary may be sent when monitoring resumes; it is not a reconstruction of unseen activity. At most one scheduled summary is attempted per local calendar day. The delivery date is saved before sending to avoid duplicate summaries after restart; failed deliveries are not replayed.

## Optional receive-payment notifications

Enable ZKas and/or Kaspa receipt notifications and the matching wallet-viewing permission. ZKas uses signed-in wallets in the unlocked vault and/or FVK wallets configured in the Bot tab. Kaspa uses only the currently open, synced wallet/account; monitoring never switches accounts. Both use incoming history records, not changes in balance. Outgoing transactions and Kaspa change/reorg/internal-transfer records are not announced as external receipts.

The first usable history sample for each wallet/account establishes a baseline. Only timestamped incoming records after that baseline are announced. Baselines and transaction IDs survive restarts, so old history and repeat polls do not create duplicate notifications. ZKas memos and receiving addresses are included when returned by the backend. These history interfaces do not provide a confirmation count; messages say so rather than claiming finality. A later reorganization can still invalidate a transaction.

ZKas polling rotates through up to two wallets per minute; Kaspa checks the current account about once a minute. Wallets must be registered and synced already. Status checks can wake a registered ZKas backend wallet, but monitoring does not create registrations, unlock vaults or perform full rescans. Each history read is bounded to 500 recent records; very high activity can exceed the window. High-volume notifications are grouped by wallet, showing up to three transaction details; complete details remain in wallet history. Notifications are not a complete accounting ledger.

Monitoring-state read/write failures stop optional notification monitoring and display an error in the Bot tab. Bot commands remain available. Allowlisted recipients must have started a private conversation with the bot. Notification state is encrypted for the current Windows user.

## Optional Telegram spending limits

Limits are off by default. When enabled, set per-payment and/or rolling 24-hour limits separately for ZKAS and KAS. Blank fields have no cap. Limits cover payment amount plus network fee, aggregate all allowed Telegram users and wallets on this manager, and do not apply to desktop sends or another computer.

A confirmed attempt reserves allowance durably before signing/submission. Repeated confirmations cannot reserve or submit twice. The encrypted spending ledger is separate from bot settings, so saving settings or restarting does not reset it. Disabling limits does not erase existing reservations; sends made while disabled are not counted. Confirmed attempts that fail or have an uncertain outcome remain counted for 24 hours. This deliberately avoids freeing budget when a transaction may have been broadcast. Cancelling an unconfirmed review does not reserve allowance. A damaged/unreadable ledger blocks limited sends rather than resetting their budget.

## Independent Telegram lock controls

**Allow vault / wallet unlock and lock from Telegram** is off by default. It works only for chains whose access checkbox is enabled. Open **ZKas wallets** or **Kaspa wallets**. If that Telegram vault is locked, the bot asks for its password and resumes the selected wallet screen after unlocking. The main menu contains no Unlock / Lock buttons. **Lock Telegram wallet** appears inside wallet screens; optional `/unlock zkas`, `/unlock kaspa`, `/lock zkas` and `/lock kaspa` commands remain available. Wallet screens and password prompts show the desktop vault lock state. Unlock prompts expire after two minutes. Send the vault password in the next message; do not append it to the command. Password messages are never echoed or logged, and the bot attempts to delete them. A deletion failure is reported. Unlock attempts are throttled to one per user every 30 seconds. Telegram cloud chats are not end-to-end encrypted; local Bot-tab unlock buttons avoid sending a password through Telegram.

Unlocking never unlocks the desktop vault. Locking a Telegram vault clears its credentials and releases its logical Telegram wallet session. All allowed users share the bot's session and permissions. Stopping the bot clears both Telegram vault sessions. Sessions are not restored automatically at startup.

The desktop and Telegram can both remain unlocked. Their Kaspa sessions use a single serialized SDK storage owner, with independent credentials, wallet/account selections, lock state and payment review tokens. A request restores the requesting session's selection before using the SDK; locking one session does not lock the other. This avoids concurrent writers to the same wallet files without requiring the manager to be locked. The node is still a separate process.

Kaspa balance display can read the same local-node UTXOs used by the address table if the SDK has not supplied a balance event. It totals all generated receive and change addresses, including multiple pages (up to 10,000 addresses for this fallback), and caches a successful total for up to five seconds. An on-chain total does not by itself authorize spending; the SDK still validates maturity and readiness.
