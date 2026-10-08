# 0.9.5-preview validation

Portable Go tests pass, including recovery from a simulated Windows log-rotation sharing violation, encrypted vault isolation and authentication, payment validation, and existing node/sharing/updater checks. The actual official Kaspa SDK offline tests verify receive indexes 0 and 1, address persistence and snapshot restore. Address/UTXO tests cover exact integer totals, separate address grouping, change outputs, unknown-address rejection and bounded details. Windows code is cross-compiled and statically checked.

The address panel uses official SDK derivation indexes and public keys, checks derived current receive/change addresses against the account descriptor, and queries the configured local node for UTXOs. Offline responses do not report fabricated zero balances. Address pages are limited to 50 rows and output details to 100 entries per address; totals include all returned outputs.

Native Windows UI sizing, the native log-sharing test, live node UTXO queries, live-funded payments/consolidation and hardware mining have not been run in this environment. No real funds were used. The log-rotation fix addresses a confirmed error-handling defect; it does not establish the cause of any particular backend exit. The final backend log lines are needed to distinguish memory exhaustion, a panic, a port conflict or another failure.
