# 0.9.6-preview — Wallet notes and key controls

- ZKas wallet status shows the backend's note count. An unavailable count is labelled rather than shown as zero.
- Change the ZKas vault password using the button at the bottom of the wallet tab.
- Export the private key for a selected Kaspa receiving address after confirming the vault password. Each key is checked against that address before display.

For an existing Kaspa wallet without a recovery secret saved in its vault, key export requests the original phrase or private key once and verifies it before saving it encrypted. An individual receiving key is not a backup of the whole wallet.

Consolidation and payment behavior are unchanged. Portable and offline wallet tests, Windows cross-compilation and static checks passed. Native Windows UI and live transactions have not been tested in this environment.
