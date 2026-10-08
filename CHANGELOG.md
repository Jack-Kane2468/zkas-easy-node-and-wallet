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
