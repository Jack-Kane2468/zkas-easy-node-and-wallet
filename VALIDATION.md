# Validation

## 0.8.5-preview

- Portable Go tests passed.
- Windows-target static analysis and the Windows x64 GUI build passed.
- The complete release archive passed checksum verification and the updater's extraction checks.
- The six bundled wallet runtime files match their pinned hashes and are unchanged from 0.8.4.

The Install / setup button and revised Settings layout have not been exercised on Windows. Native updater restart/recovery, live payments, full-chain viewing-key scans, mining and remote sharing were not retested for this release. No new vulnerability scan or independent security audit was performed.

Cross-compilation and automated tests do not establish end-to-end behavior on a Windows installation. This remains an unsigned community preview.
