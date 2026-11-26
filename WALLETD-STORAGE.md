# Wallet daemon storage and resource settings

These controls manage the disk space, RAM and CPU used by the local wallet service. Compact spent-note records stores shared nullifier data once instead of repeating it per wallet. Shared transaction-tree data avoids keeping a chain-leaf copy in every wallet scan file. Both target disk usage in the wallet data folder, especially with many wallets. They do not change coin balances or shrink the node’s blockchain database. Each advanced control has a description directly beneath it in the manager.

## Enable the two storage optimizations

1. Update the ZKas node/wallet backend from Overview → ZKas → Check node updates. The options were verified against the official v1.0.14 daemon CLI.
2. Stop ZKas services. Back up the complete manager wallet data directory while stopped; the new Open wallet data folder button locates it. Keep recovery secrets backed up separately too.
3. In Settings → ZKas → Wallet service · disk space and performance, enable Reduce wallet disk space: compact spent-note records and Reduce wallet disk space: share transaction-tree data.
4. Click Check installed service support, then Save settings.
5. Start ZKas services from Overview. Restart mining/sharing separately if needed.

The daemon reduces checkpoint duplication as wallets load and checkpoints are rewritten. Saving these settings does not immediately rewrite every dormant wallet or guarantee a particular reduction in size. Watch wallet backend logs and compare disk inventories over time. It does not request a rescan or delete wallet registrations.

Shared leaf storage is a checkpoint-format transition (v11). Builds predating that format cannot read converted checkpoints. Unchecking the option is not a rollback. Keep shared chain-tree and nullifier stores together with checkpoints in complete backups and migrations.

## Advanced controls

Zero means omit the flag and let the daemon choose its default; for the hard limit, this leaves it uncapped by that setting. A memory budget applies to resident-wallet estimates, not total process RSS. Settings apply to every wallet registration in this manager's wallet daemon.

| Control | Daemon flag |
|---|---|
| Compact nullifiers | --compact-nullifiers |
| Shared leaf storage | --shared-leaf-store |
| Soft resident-wallet limit (existing) | --max-resident-wallets |
| Hard resident-wallet limit | --hard-max-resident-wallets |
| Resident memory budget, MiB | --resident-budget-mb |
| Concurrent wallet scans | --sync-wallets |
| Concurrent checkpoint loads | --load-wallets |
| Proof CPU threads | --proof-threads |
| Concurrent proofs | --max-concurrent-proves |
| Idle RAM eviction, seconds | --idle-evict |
| Checkpoint interval, seconds | --checkpoint-interval |
| Catching-up checkpoint interval, seconds | --catchup-checkpoint-interval |
| Scan prefetch depth | --prefetch-depth |

Lower proving limits can leave CPU for a colocated node but increase payment time. Longer checkpoint intervals reduce writes at the cost of more catch-up after an unclean exit. Higher scan/prefetch concurrency can increase memory and disk pressure. Leave advanced values at zero unless you have a reason to tune them.

The built-in manager uses watch-only daemon operation (--no-custodial) and signs locally. Custodial background auto-consolidation does not apply to these wallets; existing explicit consolidation remains available. Experimental scan-engine/read-only modes, uniform transaction/fee changes, and public-listener flags were not added as generic storage controls because they alter service behavior beyond resource usage.

## Disk review and old files

Review wallet disk usage is read-only and lists checkpoint recovery copies (.scan.bak, .scan.bak.*, .scan.stale-*, .scan.divergent-*). The current upstream source includes pruning of superseded copies while retaining recovery copies. Remaining large files are not automatically disposable; retain recovery material while investigating missing funds or scan problems. Active .scan files, registrations, shared chain data and __nullifiers__.nfs are not cleanup candidates. This manager does not delete any of these files.

The inventory reports file lengths rather than physical allocated space and excludes symbolic links. Sizes may change while the daemon is running. Open wallet data folder is provided for manual inspection/backup/cleanup after services are stopped.

## Verification sources

- Official docs: https://docs.zkas.info/docs/wallet/
- Release download index: https://zkas.info/downloads/node/
- Official v1.0.14 Linux archive SHA-256 used to inspect the CLI: 433b3a5a71d06e92c2ae029ec87f06f3963a63f45f5320da991d59507b725c2a
- Radicle zkas-v1.0.14 source commit inspected: 6c2450a921d53cda4ab21f5ae7df04a134e6ff97 (zkas-walletd/src/main.rs and lib.rs).

The installed Windows binary's own help is checked at runtime rather than relying on version names alone. No upstream binaries or daemon upgrades are bundled in the manager package.
