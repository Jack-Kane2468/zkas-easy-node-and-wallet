# 0.9.18-preview — Passive wallet selection and targeted rescans

- Selecting a ZKas wallet no longer opens it automatically. Wallets explicitly opened in this manager remain eligible for status polling for 30 minutes; locking the vault clears that session state.
- Show a plain instruction to run the ZKas node when the local wallet backend is unavailable.
- Shorten the backup-section button to Show spending key.
- Add Rescan stuck wallets under Settings → ZKas → Wallet maintenance. Candidates require repeated checkpoint refusals in the recent backend log, a matching current checkpoint and a live confirmation that the wallet is opening and the node still rejects that checkpoint.
- Add wallet-specific recovery buttons beside checkpoint diagnostic messages inside the wallet-backend log view. Each button targets only one uniquely matched current registration, including external bot registrations. Ambiguous matches have no action. Matches and rejection are rechecked after confirmation; stale or resolved cases are skipped.

Rescans use the official API and retain its history-availability check. Wallet keys and registrations are kept. No daemon binaries or daemon source were changed.
