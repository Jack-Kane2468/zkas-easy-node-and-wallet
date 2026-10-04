# Wallet tools

Open **Wallet → Wallet tools**. You do not need to unlock, create or sign into a wallet. Paste an FVK or OVK, choose its type, then Start scan. Viewing keys are sensitive: they grant visibility into wallet activity, although they cannot authorize spending.

## FVK scans

A full viewing key is 96 bytes / 192 hex characters. The manager registers a dedicated view-only profile with your local wallet API. The daemon validates the key, scans the available chain history, and returns balance, spendable/maturing amounts, scan progress and transaction history. The FVK and scan cache are retained by the daemon, separately from your encrypted personal-wallet vault. Registration does not enable sharing.

The tools tab shows the default receiving address and distinct receiving addresses found in the history loaded so far. **Derive FVK receiving addresses by index** displays up to 100 addresses from a chosen index. Derived does not mean used: there are far too many possible addresses to list every unused one. The transaction table shows kind, amount, date, address, transaction ID, memo and known fees as reported by the daemon.

FVK history follows the daemon's history representation; grouped/batched sends may not enumerate every outgoing output recipient. If a profile inherited a cache with missing old history, **Rebuild this FVK scan from genesis** rebuilds only the tool profile. It does not alter your personal vault or spending keys. The daemon refuses a lossless full rescan when it cannot serve genesis. History display is capped at 100,000 rows and labels truncation; active scans can change history while pages are fetched.

Stopping the tool stops its UI queries, not the daemon's own background scan. Click Start scan again to reload current results.

## OVK scans

An outgoing viewing key is 32 bytes / 64 hex characters. This is actual ciphertext recovery, not a match against logged-in wallets. The helper uses `zakura-orchard = 1.0.1` and `zcash_note_encryption = 0.4.2`, pinned to the dependency versions in ZKas v1.0.9.

The scanner walks the local node's accepted shielded transaction stream from the pinned mainnet genesis. For transactions with available full block bodies, it fetches the accepting block and relevant merge-set blocks, matches accepted transaction IDs, and attempts authenticated Orchard outgoing-note recovery. It lists recovered output recipients, exact amounts, transaction IDs, timestamps and text memos. Pause/continue works while this manager window stays open. Start scan resets the in-memory scan; OVK keys/results/cursors are not saved to disk by this tool.

An OVK does not provide the incoming viewing key, wallet balance, ownership of an address, or the ability to identify which recovered outputs are change. Recovered output values must not be summed as net payments. Sends constructed without recoverable outgoing ciphertext cannot be recovered with an OVK. Some memo bytes may not be text and are displayed with Unicode replacement characters.

The node must serve a genesis scan anchor to begin. **Pruned or missing transaction bodies produce an incomplete result**, with a missing-body counter. A compact scan archive alone is insufficient for OVK recovery. Zero matches are not proof of no outgoing activity, particularly with missing bodies, private sends or an incorrect key. A reorganization stops the scan and requires restarting from genesis. Recovered data is read from your node; this tool does not independently verify its consensus or transaction proofs.

Full-chain scans can take time and consume node resources. Results are capped at 100,000 outputs per scan display. OVKs are passed to the local helper through private process pipes, not command-line arguments. The helper runs in the bundled Node/WASM runtime, has no filesystem preopens or network operations, and cannot sign or submit transactions. Its module and loader hashes are checked before launch.

## Validation and preview status

All ten published Orchard note-encryption vectors recovered the expected recipient and exact amount through the actual bundled WASM. Wrong keys produced no outputs; malformed bundles were rejected. Mainnet receiver encoding matches ten fixtures from the official signer. Mock RPC tests cover accepted transaction filtering, merge-set lookup and missing bodies. No live full-chain scan or Windows GUI execution was performed here.

Public fixture keys are included for testing only: do not use them to hold funds.
