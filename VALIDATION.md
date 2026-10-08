# 0.9.6-preview validation

Portable tests cover note counts (including missing/null versus zero), vault-password changes with wallet data preserved, rejection of the old password and failed-change preservation. Official Kaspa SDK offline tests check exported receiving keys against actual account addresses for multiple indexes, nonzero account numbers, BIP39 passphrases and private-key imports; mismatched accounts, indexes, addresses and recovery secrets are rejected. Windows code is cross-compiled and statically checked.

The SDK’s direct private-key data getter is not used: it aborted in an offline test. Export instead uses the SDK’s supported mnemonic/private-key derivation and address checks. An existing wallet without a saved recovery secret requires it once for address-key export. Secrets are stored in the encrypted vault and exported only through a user-requested reveal dialog.

No live transactions or native Windows UI interactions were performed. Consolidation and payment code are unchanged from 0.9.5. No real funds were used. The release package is checked for its Windows x64 GUI executable, runtime hashes, archive integrity and updater compatibility.
