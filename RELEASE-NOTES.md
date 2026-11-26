# ZKas Node Manager 0.9.33-preview

- Add Settings → ZKas → Wallet service · disk space and performance.
- Explain disk-space reduction, RAM use, CPU use, and each control’s purpose and tradeoffs directly in the interface; no prior conversation is needed.
- Add opt-in compact-nullifier and shared-leaf-storage checkboxes. Existing configurations retain their previous daemon behavior. Explain checkpoint conversion and backup compatibility before enabling shared leaf storage.
- Add advanced hard resident-wallet and memory limits, scan/load concurrency, proving thread/concurrency limits, idle RAM eviction, normal/catch-up checkpoint intervals, and scan prefetch depth. Zero preserves upstream defaults. Retain the existing soft resident limit.
- Save all settings in the manager configuration; apply on the next daemon start, including automatic service startup. Disable edits while ZKas services are running.
- Check the installed, hash-verified daemon's --help output for selected optional flags before startup, with a ten-second timeout and a clear update/disable-options error for unsupported flags. Also expose a manual compatibility check.
- Add a read-only wallet disk-usage inventory with active scan, retired checkpoint, and other/shared-store totals; list up to 1,000 retired copies with sizes and modification times. Provide a shortcut to the wallet data folder. No file cleanup is automatic.
- Preserve no-custodial operation, loopback binding, wallet token handling, and existing manager/TG session separation.

See WALLETD-STORAGE.md for usage and compatibility. Windows executable rebuilt from this source. Native Windows UI/lifecycle testing remains required.
