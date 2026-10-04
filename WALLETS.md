# Built-in wallets — start here

Wallet → My wallets manages personal ZKas mainnet wallets on this PC. You can keep multiple wallets and switch using the wallet list. Wallet activity uses your local wallet API; it does not require enabling public sharing.

## First use

1. Open Wallet > Unlock / set up vault. Choose and repeat a password of at least 12 characters.
2. Click Create wallet, give it a name, and save its 12-word recovery phrase offline. Confirm that you saved it before the wallet is added.
3. Start node/wallet services in Overview if needed. Select your wallet and click Refresh / sync. Its viewing key is registered locally; let the scan complete.
4. Copy the receiving address to receive ZKAS. To pay someone, click Send ZKAS, enter their address and amount, then review the actual fee and destination before confirming.

The **vault password** unlocks the encrypted file on this PC. The **recovery phrase/key** restores the actual on-chain wallet on another device. Keep both; a password by itself cannot restore a lost vault. If you forget the vault password, restore each wallet from its recovery backup in a new vault using a separate Windows profile, or preserve the old vault securely before deliberately replacing it. There is no password-reset backdoor.

## Import and switch

Import key / phrase accepts the official ZKas raw 32-byte spending seed/key (64 hexadecimal characters) or a recovery phrase supported by the bundled official signer. Phrases use the official ZIP-32 account derivation. Account 0 is the normal default. Only change this number if you previously created another account under the SAME phrase and know its number (1, 2, and so on). It is not the number of saved wallets and does not select a receiving address. Raw spending keys always use 0. Do not enter a wallet address or an unrelated coin's private key.

Use the wallet list to switch. Each wallet keeps its own viewing key, daemon token, extra addresses and receipts. Adding another wallet does not replace the first. A seed/key/phrase import matches an existing view-only wallet by FVK and upgrades that same profile so you can send without losing its saved addresses.

Import FVK adds a **view-only** wallet. It requires the local wallet API to validate the FVK. A view-only wallet can receive and show activity but cannot sign payments without its spending key. The public wallet API can still prepare/broadcast transactions for clients that hold their own signing keys.

## Extra receiving addresses (canary addresses)

As requested, these mean **additional receiving addresses for the same wallet**. They are not separate wallets or theft alarms.

Select a wallet and click New receiving address. The manager requests the next diversified address from the local wallet daemon, stores its index, and displays it for copying. Show all receiving addresses lists everything generated here, including the original address #0. All feed the same wallet balance. Restore from the wallet's phrase/key; the saved list of labels/indices belongs to this PC's vault.

## Send and transaction status

Send ZKAS accepts up to 8 decimal places without floating-point rounding. The maximum fee field is a ceiling, not the amount automatically charged. The daemon prepares the payment first. You then review the destination, amount and quoted fee. The official on-device signer verifies the actual bundle, recipient, amount, change and fee before signing; it will not authorize a fee above the displayed quote. Only signatures and viewing information reach the wallet daemon; the spending seed stays on the PC.

This preview sends a single transaction per action. If funds are spread across too many notes for one standard transaction, it rejects a partial-payment proposal; it does not silently split or partially pay. Preparing the first proof can take several minutes. Existing daemon scan/proof limits still apply.

A send attempt is saved before submission. If the response is interrupted, the receipt says **outcome unknown**. Check history before retrying; the manager never automatically re-submits. A transaction ID means broadcast, not final confirmation. Refresh / sync shows recent chain history and saved send receipts. An old receipt remains labelled broadcast even after confirmation; chain history is the current evidence.

Balances are hidden while the daemon reports loading. Scanning and missing-history states are labelled; payments are blocked until the wallet is ready. Newly received funds may be maturing and not yet spendable.

## FVK and OVK

Use **Wallet → Wallet tools** for independent scans without a vault login. Both arbitrary FVK scans and OVK outgoing ciphertext recovery are implemented. See WALLET-TOOLS.md for discovered/derived address lists, partial-history warnings and the scope of each key.

- **Export FVK:** full viewing key, 96 bytes / 192 hex characters. It grants incoming viewing capability and recoverable outgoing history, without spending authority.
- **Export OVK:** external Orchard outgoing viewing key, 32 bytes / 64 hex characters. It can reveal outgoing payment details where recoverable history was used. It is not a spend key or a complete balance viewer.

Exports require the vault password again. Treat viewing keys as private financial information. Copying a key or recovery secret puts it on the Windows clipboard until replaced; clipboard history/sync is outside the manager's control.

## Sign out, lock and remove

**Sign out of this wallet** disables that profile's actions and status display until Sign back in verifies the vault password. Other wallets stay available. It is a UI access state within the unlocked vault, not a separate encrypted session or a command to erase daemon state.

**Lock all wallets** closes the vault session in the manager. Closing the app also ends the session. Reopen it with the vault password. The manager does not promise secure erasure of every prior copy of a secret from operating-system/process memory, and this version does not automatically lock after inactivity.

**Remove wallet** removes its profile and saved recovery secret from the encrypted vault. It does not move/delete blockchain funds. Keep a recovery backup first. Existing viewing keys/scan data held by walletd are retained. File deletion is not guaranteed secure erasure of old disk blocks or backups.

Vault location: `%LOCALAPPDATA%\ZKasNodeManager\personal-wallets\vault.json`. AES-256-GCM encryption, PBKDF2-HMAC-SHA256 with 600,000 rounds and a fresh 32-byte salt on each save. The directory also gets a current-user/SYSTEM ACL. The vault and node configuration are separate. Uninstall retains personal wallet data.

## Bundled signer and validation

Keep `wallet-runtime` alongside the EXE when running the extracted package. It includes Node.js v24.21.0 Windows x64 and the official ZKas signer WASM/glue, pinned to zkas-wallet commit `3668c84c14691a1487eac003daba9ff68af78065`. No separate Node.js installation is needed. The manager checks component hashes before invoking the signer and passes private data through process pipes, not command-line arguments.

Tested: official signer phrase creation/restoration, raw-key import, account differentiation, a published Orchard FVK vector, invalid phrase and malformed-payment rejection; all ten published Orchard OVK vectors; encrypted vault round trip, wrong-password/tamper rejection, exact amounts and local mock API requests.

Not executed here: native Windows GUI/ACL/process behavior, a fully prepared real ZKas payment, or an on-chain transfer. This is an unsigned preview. Do not treat a successful build or mocked API test as an end-to-end wallet audit.

Sources:
- https://github.com/firecash/zkas-wallet/tree/3668c84c14691a1487eac003daba9ff68af78065/src/signer
- https://github.com/firecash/zkas-rusty/blob/zkas-v1.0.9/zkas-walletd/src/lib.rs
- https://github.com/zcash/orchard/blob/main/src/keys.rs
- https://github.com/zcash/orchard/blob/main/src/test_vectors/keys.rs
- https://docs.rs/zcash_spec/0.2.1/zcash_spec/struct.PrfExpand.html

## Show history

Select a signed-in wallet under Wallet → My wallets, then click **Show history**. The window is labeled with that wallet’s name and address. It shows transaction type, local time, amount, address, transaction ID, available memo and known fee. Select a row to read the full details. Previous/Next navigate pages of up to 500 entries; Refresh returns to the newest page. Closing cancels the active request.

Memo is the optional encrypted message attached to a payment, such as an invoice reference. Blank means no memo was returned for that entry. Scanning or missing-block warnings mean history may be incomplete. This window reads existing backend history; it does not register a new wallet or start a rescan. If the profile is unavailable, close the window and use Refresh / sync.
