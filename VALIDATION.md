# 0.9.9-preview validation

Portable Go tests cover existing node/wallet/mining behavior and bridge release validation: exact repository/asset matching, required SHA-256, stable-release selection, duplicate assets, download size and downgrade refusal. The official v1.0.12 bridge and v1.0.11 core Windows archives were checksum-verified and extraction-tested. Windows code is cross-compiled and statically checked.

The bundled v1.0.47 signer matches upstream commit 8689fb4da9fa399c0a0f68ce49e11be98ad064f0. Offline tests cover public address, FVK and seed/account derivation vectors, the manager adapter and malformed-payment rejection. These do not constitute a successful live transaction or a security audit of upstream WASM.

Existing OVK decryption vectors and copies with the named-anchor trailer pass. Truncated/extra trailers are rejected. This adapter is for decryption only; it does not verify consensus or change bundles submitted for signing. Existing mining statistics tests cover separate ZKas/Kaspa counters, hashrate units, malformed responses and bounded worker output.

Package checks cover the Windows x64 GUI EXE, runtime hashes, archive integrity, SHA256SUMS and updater extraction. No native Windows UI, live mining hardware, live upgrade/restart or payouts were tested. Wallet vault formats, payout field order and local-network sharing are retained.
