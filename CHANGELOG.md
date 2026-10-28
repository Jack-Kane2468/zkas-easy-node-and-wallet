# ZKas Node Manager 0.9.28-preview

- Opening a locked Telegram wallet now prompts for the vault password, then resumes the selected wallet screen. Existing remote-unlock and per-chain permission settings still apply.
- Removed Unlock / Lock controls from the main menu. Lock Telegram wallet is available inside wallet screens.
- Wallet screens and password prompts report whether that chain's desktop manager vault is locked or unlocked.
- Removed the requirement to lock the manager before using Kaspa in Telegram. Desktop and Telegram maintain independent logical sessions through one serialized SDK storage owner. Wallet/account selections, credentials, lock state and payment review tokens remain separate. Opening in Telegram never unlocks the desktop.
- Retains the 0.9.27 balance fallback, copy buttons, separate chain permissions and optional tray background mode.

## Use

Stop the previous manager, extract the included Windows ZIP and run ZKasNodeManager.exe with its wallet-runtime folder. In Bot, enable the desired chain and remote lock/unlock if you want password prompts in Telegram. Open Wallets in Telegram. Local Bot-tab unlock buttons remain available when remote unlocking is disabled. Existing saved settings and application data are retained.

## Validation

Core Go tests and Windows-target vet passed; Windows amd64 GUI executable built with Go 1.26.8 and the embedded manifest. Official SDK offline tests cover two logical sessions on the same storage, independent locks and selections, rejected-password/filename isolation and address persistence. Adapter tests cover session-bound payment tokens, switching and replay rejection. Telegram tests cover prompted opening, resumption, wrong passwords, permission checks and main-menu cleanup.

Native Windows UI/tray behavior, live Telegram and live-node transactions have not been tested in this Linux environment.


# ZKas Node Manager 0.9.27-preview

- Kaspa balance header falls back to local-node UTXOs across all generated receive/change addresses instead of waiting indefinitely for an SDK balance event. Exact integer arithmetic; signing readiness remains SDK-controlled.
- Separate ZKas and Kaspa Telegram access checkboxes; direct commands, menus, alerts and sends obey the selected chain.
- Optional remote vault lock/unlock with Telegram-only sessions, expiring prompts, rate limiting and attempted password-message deletion. Local Bot-tab unlock buttons are also available.
- Stop bot when manager closes defaults ON. Uncheck before Save and start bot to keep it in the tray. Reopen or quit through the tray icon. Desktop wallets lock when the window closes into the tray.
- Copy-address / transaction buttons, fewer duplicate wallet labels, and no redundant selected-account button.

## Use

Extract the complete Windows ZIP and run ZKasNodeManager.exe with its wallet-runtime folder. Stop the older manager before replacing files. Existing application data stays in its existing location. Select the desired chain permissions in Bot, choose shutdown behavior, and Save and start bot. Unlock Telegram sessions from the Bot tab or enable remote unlock.

## Validation and limits

Built for Windows amd64 with Go 1.26.8 and an embedded Windows manifest. Core Go tests, Windows-target vet, Kaspa SDK offline integration tests, status/balance tests and address tests passed. The Windows tray UI and a live Telegram / live-node session have not been exercised in this Linux build environment. Background mode keeps the manager process resident; it is not a separate Windows service. The SDK's wallet storage is used by only one desktop/Telegram Kaspa runtime at a time.


# v0.9.26 Preview — Bot Logs and Kaspa Refresh Handling

- Add Bot log to the Logs tab: startup/shutdown, polling and delivery failures, request progress, Kaspa refresh stages, timeout diagnostics and balance readiness. Logs rotate at 2 MB and omit Telegram content, tokens, wallet names, addresses and signing material.
- Use the same local-wallet availability check for the manager and Telegram; reject old buttons targeting unavailable wallets.
- Refresh an already connected Kaspa wallet without reconnecting/reactivating it or rewriting the encrypted vault backup on every refresh.
- Give Telegram Kaspa open/refresh requests a 40-second request deadline, including waiting for the subprocess lock. A queued reader never terminates an operation already in progress. An active timed-out request stops its wallet subprocess; it is never replayed automatically.
- Show a Telegram refresh-progress message. Distinguish disconnected, node-not-synced and node-synced/wallet-balance-pending states. Surface SDK and missing-UTXO-index errors and handle unavailable balance events without treating them as zero.

The reported missing live wallet balance is not confirmed resolved. The updated status and Bot log expose the relevant state without pretending a synced node guarantees the wallet has received its balance.

Official node, walletd, Kaspa SDK binaries and wallet/vault formats are unchanged.

# v0.9.25 Preview — Kaspa Wallet Startup Fix

- Fix a Kaspa wallet process crash when reconnecting or opening another wallet. The manager now starts SDK processing once per subprocess and reuses it across wallet changes and connection attempts.
- Remove the permanent wallet error-details button. Wallet and manager operation error popups now include selectable details and a Copy details button.
- Preserve Telegram behavior: typed commands receive fresh replies; button interactions edit their attached menus.

No node, walletd, official Kaspa SDK, wallet format or vault format changes. Existing wallets and settings are retained. Payment requests are never automatically retried.

Validation: reproduced the original SDK trap with the same five WebAssembly stack locations; added a regression test for repeated connection attempts and opening another wallet after processing starts. Internal Go tests, offline SDK tests, Windows cross-build and vet pass. Windows GUI execution and live-node/funded transactions were not tested on the Linux build host.

# v0.9.24 Preview — Telegram Replies and Kaspa Crash Details

- Typed Telegram commands and payment inputs receive a fresh reply at the bottom of the chat. Button clicks continue editing the message containing that button. A long-running typed request may update its new progress reply with the result.
- Kaspa wallet errors retain safe SDK stack locations, fixed runtime error categories and the last open/start/connect/activate stage. A Show / copy wallet error details button makes the report copyable.
- Keep bounded raw stderr only in process memory; do not include raw panic values, private keys or full local paths in the crash report.

The reported Kaspa WebAssembly process crash is not yet diagnosed or claimed fixed. These changes preserve evidence needed to investigate it; restarting alone did not resolve the reported failure. No node, walletd or official Kaspa SDK binaries were changed.

Validation: Telegram command-versus-button routing test, existing internal tests, Windows build/vet and offline SDK tests. Live Windows GUI and the reported crash have not been reproduced here.

# v0.9.23 Preview — Kaspa Wallet Process Recovery

- Detect stopped Kaspa wallet subprocesses instead of retaining a dead process handle indefinitely.
- Opening the selected wallet starts a fresh process when necessary; wallet operations and payments are not automatically replayed.
- Opening a desktop Kaspa wallet also connects it to the configured local wallet RPC. Opening an already-selected wallet reconnects it.
- Clear stale open/balance state when the wallet process exits, so unavailable wallet actions are no longer presented as ready.
- Require a startup-ready response and include a safe diagnostic category and exit code when the process stops. Raw runtime diagnostics and wallet secrets are not forwarded or saved to disk.
- Retain v0.9.22's startup correction and optional Telegram monitoring/spending-limit features.

The recovery defects are addressed; the original Windows subprocess exit has not been reproduced. If it recurs, the new error provides additional evidence. Official node, walletd and Kaspa SDK binaries are unchanged.

Validation: internal Go tests, Windows cross-build/vet, and offline Kaspa SDK wallet tests including continued operation after a failed connection attempt. Live Windows GUI and funded sends remain untested here.

# v0.9.22 Preview — Startup Fix, Bot Alerts and Spending Limits

- Fix manager startup failure (`value out of range`) caused by an integer default in the Bot tab's numeric input. Numeric defaults now use the type required by the Windows UI library; a regression check covers all manager NumberEdit literals.
- Optional mining-down alerts for persistent worker/hashrate shortfalls, statistics API outages and unexpected bridge stops, with recovery notifications.
- Optional daily Telegram summary at a configurable local time, with honest observation coverage and saved delivery state.
- Optional receive-payment notifications for available ZKas vault/FVK wallets and the currently open Kaspa account, with history baselines and duplicate suppression.
- Optional per-payment and rolling 24-hour Telegram spending limits, separately for ZKAS/KAS, including fees. Reservations persist across restarts.

All four features default off. Configure them in the Bot tab. The manager's Kaspa runtime adapter now reports structured fee and incoming-history data; official node, walletd, signer and Kaspa SDK binaries are unchanged.

Includes the Windows x64 manager ZIP, source and SHA256SUMS.txt. Automated Go tests, Windows cross-build/vet and offline Kaspa runtime tests are covered. Live Windows UI, Telegram delivery, mining events and funded transactions have not been tested in this environment.

# v0.9.21 Preview — Telegram Alerts and Wallet Sending

- Optional alerts for bridge-reported blocks, unexpected ZKas/Kaspa node stops, lost sync, RPC outages and stalled chain progress.
- Guided Telegram sending for unlocked ZKas and Kaspa wallets, with payment review and a simple Confirm button. The new spending permission is off by default and separate from local payment requests.
- Open wallets and select accounts from the unlocked Kaspa vault directly in Telegram.
- Update menus and buttons in place; alerts remain separate messages.
- Single-use payment confirmations, wallet/account identity checks and saved submission receipts.

Includes the Windows x64 manager ZIP, source and SHA256SUMS.txt. No node, walletd, signer or bridge binaries were modified by this manager update.

Validation: internal Go tests and Windows cross-build/vet checks. Live Windows UI, Telegram delivery and funded sends have not been exercised for this release.

# 0.9.20-preview — Telegram menus and simple confirmations

- Button menus for node status/controls, mining statistics/controls, vault wallets and FVK-only wallets, showing only enabled features.
- Named wallet buttons with pagination, wallet action submenus, Refresh and Main menu navigation. Existing commands remain available.
- Confirm / Cancel buttons and plain `confirm` / `cancel` replies replace typing confirmation codes. Confirmation buttons remain bound to their original request and user; stale buttons cannot approve a newer action.
- New bot messages allow copying addresses, history and other text normally.
- Telegram’s Menu shortcut opens the manager menu. Owner pairing, allowed IDs and permissions are preserved.

The bot still runs while the manager is open. Payment requests still require local review. Walletd and node/mining binaries are unchanged.

# 0.9.19-preview — Telegram bot controls

- New Bot tab for a user-supplied Telegram bot token, owner-only pairing and an optional numeric user-ID allowlist.
- Independent permissions for node status, mining statistics, node control, mining control, unlocked vault wallet viewing, separate FVK wallets, and locally reviewed ZKas payment requests.
- Private chats only; unknown users and groups receive no replies. Service/scan/payment requests require an expiring, single-use confirmation bound to the requesting user.
- Windows-user encryption for saved tokens, viewing keys and access settings. No private-key or recovery-phrase export over Telegram.
- Runs while the manager is open. Closing/updating the manager stops the bot; restart it explicitly from Bot. Existing node and mining services continue independently.

This release changes the manager only. It does not replace or modify walletd, change upstream download sources, or include the proposed walletd memory fixes.

See BOT.md for supported commands and VALIDATION.md for testing limitations.

# 0.9.18-preview — Passive wallet selection and targeted rescans

- Selecting a ZKas wallet no longer opens it automatically. Wallets explicitly opened in this manager remain eligible for status polling for 30 minutes; locking the vault clears that session state.
- Show a plain instruction to run the ZKas node when the local wallet backend is unavailable.
- Shorten the backup-section button to Show spending key.
- Add Rescan stuck wallets under Settings → ZKas → Wallet maintenance. Candidates require repeated checkpoint refusals in the recent backend log, a matching current checkpoint and a live confirmation that the wallet is opening and the node still rejects that checkpoint.
- Add wallet-specific recovery buttons beside checkpoint diagnostic messages inside the wallet-backend log view. Each button targets only one uniquely matched current registration, including external bot registrations. Ambiguous matches have no action. Matches and rejection are rechecked after confirmation; stale or resolved cases are skipped.

Rescans use the official API and retain its history-availability check. Wallet keys and registrations are kept. No daemon binaries or daemon source were changed.

# 0.9.17-preview — ZKas wallet selector fixes

- Move Show spending key and account index into Backup and viewing keys.
- Pause background interface updates while the ZKas wallet dropdown is open.
- Apply a wallet selection after the list closes, suppress duplicate selection events and preserve the prior selection when canceled.
- Guard programmatic wallet-list changes and avoid reapplying unchanged enabled states and balance text.

Includes the spending-key display, per-wallet rescan, checkpoint diagnostics and received-output memo support introduced in 0.9.16. No node or wallet daemon changes.

# 0.9.16-preview — ZKas wallet controls and received memos

- Added spending-key/account-index display, selected-wallet rescan and checkpoint diagnostics.
- Displayed received-output memos and address/amount breakdowns.

# 0.9.15-preview

- Settings → ZKas now offers a rescan for all registered wallets, including bot/application wallets, using the official backend API. Keys and registrations are retained.
- Displays accepted request counts separately from scan completion and reports partial failures.

# 0.9.14-preview

Restores the v0.9.11 manager behavior, retaining the Kaspa wallet-selection improvements. ZKas components are downloaded from official upstream release assets. No modified wallet daemon is bundled. Node update checks refer only to official upstream node releases.

# 0.9.11-preview

Improved Kaspa wallet switching and clarified wallet/account selectors.

# 0.9.10-preview — Kaspa Overview updates

- Added **Check manager updates** to Overview → Kaspa. Both Overview pages share the same manager update channel and preview-release setting.
- Added **Check node updates** to Overview → Kaspa. It checks the latest stable official kaspanet/rusty-kaspa Windows x64 release, shows installed/available versions, and verifies SHA-256 before stopping the node.
- Updates retain the Kaspa database, wallet vault, ports, sharing configuration and mining settings. Only the node executable is selected for replacement; bridge updates remain in Mining.
- A running Kaspa node restarts after updating. A stopped node stays stopped. Active Kaspa or merged mining is paused; start it again when Kaspa is ready. ZKas services and ZKas-only mining are unaffected.
- Repair avoids replacing an updated Kaspa node with an older installation pin.

Windows cross-build/static checks and portable tests pass. Native Windows upgrade/restart and live mining were not tested in the build environment.

# 0.9.9-preview — Mining bridge updates and ZKrise wallet support

Each Mining page now has **Check for bridge updates…**, with installed version information. Kaspa and merged bridges check their official GitHub stable releases; the original ZKas-only bridge follows the installed ZKas core release. Downloads are verified before a running bridge is stopped. An updated running bridge restarts with saved addresses, worker names, ports and LAN settings. Nodes and wallets continue running during a bridge-only update.

New merged bridge installations use solo-dual-mode v1.0.12. The bundled official wallet signer is updated to zkas-wallet v1.0.47, and OVK viewing supports the named-anchor bundle format introduced by ZKas core v1.0.11. Merged-mining payout field order is unchanged: ZKas address as username (optional dot-suffixed worker), Kaspa address as password.

For ZKrise, update the manager, then use **Overview → Check node updates** for the ZKas node and enabled wallet backend, then **Mining → Check for bridge updates…** for the bridge in use. Updating the manager alone does not update those separately installed components.

Portable tests, offline wallet derivation/viewing vectors, Windows cross-build/static checks and package verification pass. Native Windows operation, physical miners and live payments were not tested in the build environment. See VALIDATION.md.

# 0.9.8-preview

- Show live mining statistics inside ZKas, Kaspa and Merged mining pages: TCP sessions, recent workers, estimated hashrate, accepted shares, stale/invalid shares and block counts.
- Show separate ZKas and Kaspa block counts for merged mining; unavailable counters are labelled rather than guessed.
- Read the active bridge's statistics port, including ZKas-only running through the merged-capable bridge. Poll only while Mining is visible, with bounded output and request timeouts.
- Add an optional merged-mining worker name to copied miner settings. Username remains the ZKas address and password remains the Kaspa address.

# 0.9.7-preview

- Export the selected Kaspa wallet's recovery phrase after vault-password confirmation and account verification; include its BIP39 passphrase when present.
- Separate local-network sharing controls for ZKas and Kaspa, with selectable local IPv4 address, running/stopped state and copyable API endpoints.
- LAN access is limited to the selected private subnet and Windows Private/Domain profiles. Stop sharing closes connections; uninstall removes its firewall rules.
- Keep the Logs row inside the README's tab table.

# 0.9.6-preview

- ZKas wallet displays the backend’s note count alongside balance and scan progress.
- Change the ZKas vault password from the bottom of My wallets.
- Export the private key for a selected Kaspa receiving address after vault-password confirmation. The derived key must reproduce the selected address before it is shown.

# 0.9.5-preview

- Indexed Kaspa receiving/change addresses with address-specific unspent totals, UTXO counts and output details.
- Address derivation is checked against the official SDK account descriptor. Disconnected address queries show unknown balances instead of zero.
- Flexible two-column wallet controls and wrapping status text.
- Log reading permits Windows file rotation; service logging recovers from temporary rotation failures.
- Wallet status polling is limited to the visible ZKas wallet page. Backend exit details appear in Overview.
- Starting a stopped ZKas backend uses the current service host without reinstalling the manager or stopping a running node.

# 0.9.4-preview

- Separate encrypted Kaspa wallet vault.
- Complete encrypted Kaspa vault backups/restores, vault password change, receiving-address records and removal with encrypted recovery copies.
- Correctly describe Kaspa receiving addresses as transparent, not shielded canary addresses. Private-key accounts cannot generate additional addresses.
- Kaspa sending waits for a synchronized balance, retains fee review and writes a durable receipt before submission.
- Optional encrypted ZKas send memos (512 UTF-8 bytes) and note consolidation with fee confirmation, local signing and saved receipts. Consolidation performs one round, requires at least three inputs, and refuses malformed/over-fee responses.
- Preserve existing node configuration, wallet vaults, SDK wallet files and running services.

# 0.9.3-preview

- Shared Overview, Wallet, Sharing and Settings pages with ZKas/Kaspa subtabs.
- Exactly three fixed mining tabs: ZKas, Kaspa and Merged. Recognize existing bridge installations and keep individual install/start/stop/uninstall controls.
- Reuse gRPC status streams; remove the Kaspa animated loading bar and mining-mode dropdown.
- Kaspa node purposes, data-folder/port settings, independent sign-in startup, repair and uninstall.
- Kaspa peer sharing and HTTPS/WSS or personal/public Tor access, isolated from ZKas sharing.
- Local wallet and mining connections follow Kaspa port settings. Preserve existing defaults and data.

# 0.9.2-preview

- Graphical Kaspa wallet: create/import, encrypted storage, wallet/account switching, receive addresses, local-node balances, reviewed sending, history and encrypted backup restore.
- A single Mining tab includes original ZKas mining and Kaspa/merged controls. Original mining configuration remains in place.
- Separate ZKas/Kaspa settings, including an adjustable Kaspa cache memory scale.
- Preserve running services, existing wallets and blockchain data during manager replacement.

# Changelog

## v0.9.21 Preview — Telegram Alerts and Wallet Sending

- Optional alerts for bridge-reported blocks, unexpected ZKas/Kaspa node stops, lost sync, RPC outages and stalled chain progress.
- Guided Telegram sending for unlocked ZKas and Kaspa wallets, with payment review and a simple Confirm button. The new spending permission is off by default and separate from local payment requests.
- Open wallets and select accounts from the unlocked Kaspa vault directly in Telegram.
- Update menus and buttons in place; alerts remain separate messages.
- Single-use payment confirmations, wallet/account identity checks and saved submission receipts.

Includes the Windows x64 manager ZIP, source and SHA256SUMS.txt. No node, walletd, signer or bridge binaries were modified by this manager update.

Validation: internal Go tests and Windows cross-build/vet checks. Live Windows UI, Telegram delivery and funded sends have not been exercised for this release.


## 0.9.1-preview

Adds ZKas-only shared mining, state-aware Kaspa/mining controls, visible sync activity/details and access to the upstream Kaspa wallet console. See release notes for the wallet UI limitation.

## 0.9.0-preview

Optional Kaspa node and developer APIs; shared merged/Kaspa-only mining with payout checks, LAN setup, dashboard/log access and uninstall. Built from 0.8.8, preserving its bounded incremental logs. See KASPA.md for scope and limitations.

## 0.8.8-preview

- Replace full-document log refreshes with incremental text and color updates.
- Disable log undo history and skip unchanged or inactive work.
- Reduce hidden-panel polling while retaining a slow wallet keepalive.

## 0.8.7-preview

- Add a colored log viewer with bottom-following, reading-position retention, Latest and Pause controls.
- Increase the recent-log window to 128 KiB.

## 0.8.6-preview

- Hide Overview setup controls after installation; retain repair in Settings.

## 0.8.5-preview

- Add an Overview installation/setup shortcut and bring installation controls to the top of Settings.
- Simplify public documentation and remove handoff instructions.

## 0.8.4-preview

Adds verified in-app manager updates, preview channel selection, side-by-side version installation and startup recovery. Existing running services and user data are retained.


## 0.8.3-preview

- Add Show history for the selected wallet, with paginated transactions, full row details and available memos.
- Verify the wallet API profile identity before displaying history.
- Explain the Memo field in the history window.

## 0.8.2-preview

- Prepare a self-contained GitHub source repository and complete Windows build pipeline.
- Upgrade the Go release toolchain and networking dependencies following vulnerability scanning.
- Add pinned runtime bootstrap, source provenance, contribution/security guidance and release checks.
- Retain the original default appearance, wallet tools, bridge uninstall and plain-language account-number explanation.

## 0.8.1-preview

- Restore the original default Windows appearance.
- Keep mining-bridge uninstall in both Mining and the main uninstall.
- Explain account number during wallet import; normally use 0.
