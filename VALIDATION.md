# Validation — v0.9.28-preview

Passed in Linux with Go 1.26.8:
- `go test ./internal/...`
- `GOOS=windows GOARCH=amd64 go vet ./...`
- Windows amd64 GUI build with an embedded app manifest.
- `node scripts/test-kaspa-wallet.cjs` (official SDK, public vectors, offline).
- `node scripts/test-kaspa-status.cjs` (UTXO fallback totals, multi-page exact sums, cache, disconnect/lock, real zero vs missing balance).
- Address grouping and incoming-notification tests.
- Telegram chain-permission, unlock-expiry/cancel/revocation, prompted opening and resumption, wrong-password handling, and menu tests.
- Official SDK tests with desktop and Telegram logical sessions: independent locks, wallet selections, shared address persistence and failed-open isolation.
- Session-bound payment token isolation, switching, and replay rejection using a mock SDK (no broadcasts).

Not exercised here: native Windows UI/tray lifecycle, live Telegram API calls, live-node balance queries or transaction broadcasting. This preview requires a Windows smoke test before operational use.
