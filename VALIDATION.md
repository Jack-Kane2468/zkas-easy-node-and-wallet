# Validation: 0.8.3-preview

The portable Go regression suite passed, including new selected-wallet history tests for API-token isolation, page offsets, exact amounts/memos, and rejecting missing/loading/mismatched wallet profiles before requesting history. Windows-target go vet and the Windows x64 GUI build passed.

The original upload was truncated after 287 complete ZIP entries. Every recovered entry passed its CRC check. The nested Windows release was intact and CRC-verified; its wallet runtime supplied the missing outer runtime files. All six runtime hashes are checked against the embedded manifest before packaging. Runtime binaries and dependency versions are unchanged from 0.8.2.

Native Windows history-window behavior, live wallet history, payments and mining were not executed here. Existing preview limitations remain. The prior preparation results below describe 0.8.2; its vulnerability scan was not rerun for this UI update.

## Prior validation: 0.8.2-preview

## Checks performed for this package

- 26 portable Go tests passed with the updated dependencies. The optional live-release archive fixture test was skipped.
- Windows-target go vet passed. The Windows x64 GUI EXE and Windows lifecycle test binary compiled with Go 1.26.8. The lifecycle test binary was not executed here.
- govulncheck v1.8.0, targeting Windows/amd64, reported no vulnerabilities after upgrading gRPC to 1.83.2 and updating the Go networking dependencies. This is a Go dependency check, not an audit of the full wallet, native runtime or upstream daemons. Results are a snapshot of the database at preparation time.
- All ten public Orchard recovery vectors passed with the actual viewing WASM. Wrong keys yielded no outputs and malformed input was rejected. This ran under Node on Linux, not the bundled Windows node.exe.
- PowerShell parsed the build/bootstrap scripts successfully. The release-packaging script was also exercised against the cross-compiled Windows EXE. The runtime bootstrap ran from an absent node.exe, downloaded the pinned official file and verified all six runtime hashes. It also rejected an intentionally changed runtime file. Script validation/bootstrap ran under PowerShell on Linux; the complete Windows build script still needs its Windows CI run.
- The two bundled upstream signer files match firecash/zkas-wallet commit 3668c84c14691a1487eac003daba9ff68af78065 byte for byte. Wallet signing WASM and its interface are unchanged from the supplied prior version.
- Go license notices were refreshed from the 11 linked dependency modules. Runtime/helper notices and provenance accompany the source and complete release.
- Packaging checked the EXE header/Windows GUI target, runtime hashes, ZIP integrity and absence of live application data. A source scan found no apparent private credentials; intentional public test vectors are included and must never be funded.

## Checks still required on Windows

Native UI behavior, first installation, updating an existing installation, UAC/firewall changes, live mining install/uninstall and HTTPS/Tor access were not run in this environment. The provided GitHub workflow runs process lifecycle tests with a fake node after upload; it has not been executed on GitHub as part of this preparation.

No real on-chain payment, live full-chain FVK/OVK scan or live mining session was performed here. A mock/API/vector test does not substitute for those checks. Use RELEASE-CHECKLIST.md and keep the release marked as a community preview with these limits until validated.

This package has not been signed, independently audited, published to GitHub or certified for production use.
