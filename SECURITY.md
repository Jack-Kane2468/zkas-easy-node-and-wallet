# Security and privacy

This is an unsigned, unaudited community preview. A passing build or dependency scan is not an audit and does not establish that real payments or public hosting are safe. Known validation limits are listed in VALIDATION.md.

## Report privately

For a suspected vulnerability, use this repository's **Security → Report a vulnerability** once the maintainer enables GitHub private reporting. Do not post exploit details or private wallet information in a public issue. If private reporting is unavailable, open a content-free request asking the maintainer to enable it; do not attach secrets or an exploit.

Include affected version, Windows version, a minimal reproduction with disposable test data, impact and any proposed fix. There is no guaranteed response time. The latest preview is the maintenance target; old builds are not promised security updates.

## What to keep private

Never upload your recovery phrase, spending key, vault, local application folder, API credentials, TLS private key or Tor identity. FVKs/OVKs and wallet addresses can disclose financial activity. Remove them and public IPs from screenshots and logs before sharing. Repository fixtures are public test vectors and must never receive funds.

## Trust boundaries

The GUI signs using a checksum-pinned local runtime. Runtime hashes detect unexpected bytes; they are not digital signatures or protection from a compromised operating system. The encrypted vault protects stored data; an unlocked wallet and clipboard remain accessible to malware running as you.

The node, wallet daemon, Tor and mining bridge are separate upstream programs. Their releases and your PC's security remain part of the trust model. Node/bridge downloads use upstream release checksums; the project does not independently certify those binaries.

Local APIs are loopback-bound by default. Sharing must be deliberately enabled. Public registration can consume disk/CPU; gateway limits do not replace external capacity management, monitoring and abuse protection. Tor identity reuse and HTTPS certificates are retained with user data on uninstall.

Local-network sharing binds one chosen private IPv4 address and filters clients to its subnet, with Windows Private/Domain firewall rules. It shares unauthenticated, unencrypted node RPC with trusted devices on that subnet. Do not forward LAN ports to the internet; use the separate HTTPS/Tor features for remote access. LAN wallet REST still blocks seed/custodial/admin endpoints.

## Telegram bot

Bot access is opt-in and limited to private chats from configured numeric user IDs. Owner pairing requires a short-lived random code plus approval in the manager. Owner-only mode ignores additional IDs. All allowlisted users otherwise share the selected permissions. Telegram bot chats are cloud chats, not end-to-end encrypted. Viewing permissions can reveal financial history to Telegram and allowed users.

Saved bot settings are encrypted using Windows DPAPI for the current user. This does not protect against malware running as that same user. Tokens, recovery phrases, spending keys and FVKs are never included in bot replies by the supported commands. Raw daemon errors and arbitrary logs are not forwarded. No arbitrary shell execution, remote key export, destructive maintenance or remote configuration endpoints are exposed.

The bot stops with the manager by default. An explicit setting keeps it in the system tray when the desktop window closes; desktop vaults lock while Telegram sessions remain independent. It does not automatically start after a restart. Queued pre-start messages are discarded. Turning off access requires stopping the bot; edit permissions/IDs and restart it. An already-started graceful service stop is allowed to finish. Revoking a Telegram bot token in BotFather also revokes that token at Telegram.

## Telegram spending

The optional Send from Telegram permission grants allowed users access to spend from independently unlocked Telegram vault wallets. It is separate from local-review payment requests and defaults off. Telegram chats carry payment details and are not end-to-end encrypted. Reviews are short-lived and single-use, and submission errors are not retried. Keep the bot allowlist limited to people authorized to spend when this permission is enabled.

Optional Telegram spending limits are enforced per chain across all allowed users/wallets on this installation, including fees. Encrypted reservations are saved before signing/submission and retained for 24 hours even after uncertain failures. Limits do not govern desktop sends, sends while limits are disabled, or other installations. Receive notifications and summaries expose selected financial activity through Telegram and must be enabled explicitly. Notification state is encrypted; already-delivered Telegram messages are not erased by locking a vault.

Remote lock/unlock is separately opt-in and chain-scoped. A Telegram unlock cannot populate the desktop vault session. Vault passwords entered into Telegram traverse Telegram cloud chats; deletion is attempted but is not an erasure guarantee. Use local Bot-tab unlock controls to keep passwords off Telegram.

Desktop and Telegram Kaspa sessions can remain unlocked together. A single serialized SDK worker owns wallet storage; session selection and pending payment tokens are isolated. A failed open cannot inherit another session's open wallet. Locking a session drops its credentials without dropping the other session.
