# Viewing-only Orchard recovery helper

This small WASI reactor reconstructs Orchard actions from the ZKas v1.0.9 wire format and calls upstream `try_output_recovery_with_ovk`. It does not implement new cryptographic primitives, sign payments, verify consensus/proofs, or access the network. The Go manager supplies only payloads matched to the node's accepted transaction stream. Node.js loads the module with no filesystem preopens.

Dependencies are pinned by Cargo.lock to the ZKas v1.0.9 lockfile's compatible versions. The helper is ISC licensed; the manager is MIT licensed. Dependency notices are in THIRD-PARTY-LICENSES.

Rebuild with Rust 1.91.0 and its wasm32-wasip1 target:

    cargo +1.91.0 build --locked --release --target wasm32-wasip1

Copy target/wasm32-wasip1/release/zkas_view_tools.wasm to the package's wallet-runtime/view-tools.wasm, then update the embedded manager runtime hash before rebuilding the manager. The regular Go build uses the prebuilt module shipped in the complete ZIP and does not require Rust.

Run public vector tests with:

    node check-vectors.mjs

The test accepts the usual source layout or Source/view-tools in the complete ZIP. Fixtures use upstream Orchard note-encryption vectors wrapped in a synthetic ZKas action/bundle header with a public verification key. They test decryption, not proof/signature validity. Never fund any fixture key. Sources: ZKas v1.0.9 shielded-core/{wallet.rs,bundle.rs}; zakura-orchard 1.0.1 src/test_vectors/note_encryption.rs (originally Zcash test vectors).
