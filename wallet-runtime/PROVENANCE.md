# Runtime provenance

The manager verifies these runtime files before using them. Expected SHA-256 values are in `cmd/zkas-manager/wallet_runtime_hashes_windows.go`.

| File | Origin |
| --- | --- |
| node.exe | Node.js 24.21.0, Windows x64, https://nodejs.org/dist/v24.21.0/win-x64/node.exe. Its pinned hash was checked against that release's official SHASUMS256.txt. Downloaded by scripts/prepare-runtime.ps1; not committed to git. |
| official-signer.mjs | Unmodified src/signer/firecash_signer.js at firecash/zkas-wallet commit 3668c84c14691a1487eac003daba9ff68af78065; renamed for Node ESM loading. |
| official-signer.wasm | Unmodified src/signer/firecash_signer_bg.wasm at the same commit. |
| signer.mjs | This manager's JSON stdin/stdout adapter for the upstream signer. |
| view-tools.mjs | This manager's Node/WASI adapter; no filesystem preopens. |
| view-tools.wasm | Built from this repository's view-tools source and pinned Cargo.lock; see view-tools/README.md. |

Pinned signer source: https://github.com/firecash/zkas-wallet/tree/3668c84c14691a1487eac003daba9ff68af78065/src/signer

The two upstream signer file hashes were compared directly with that commit when preparing this repository. This proves byte identity, not a source-level audit or reproducibility of the upstream WASM build. The Go pipeline uses that prebuilt WASM.

NODE-LICENSE.txt includes Node's notices. SIGNER-LICENSE.txt retains upstream ISC attribution. Viewing-helper dependency notices are in view-tools/THIRD-PARTY-LICENSES (also included in runnable releases). Adapters follow the manager's MIT license. Do not remove upstream notices when redistributing.

## Kaspa wallet SDK

Official `kaspanet/rusty-kaspa` v2.1.0, `kaspa-wasm32-sdk-v2.1.0.zip`. Archive SHA-256: `ba674e109ff5dd8bedc4dc2ee8a5ecdf4b600b1178a541d77888ec58310b6124`. `nodejs/kaspa/kaspa.js` is distributed unchanged as `kaspa.cjs`; `kaspa_bg.wasm` is unchanged. License: KASPA-LICENSE.txt. `kaspa-wallet.cjs` is the manager IPC adapter and is included in the runtime hash manifest.
