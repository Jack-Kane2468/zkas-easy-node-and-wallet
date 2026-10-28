# ZKas Node Manager 0.9.28-preview

- Opening a locked Telegram wallet now prompts for the vault password, then resumes the selected wallet screen. Existing remote-unlock and per-chain permission settings still apply.
- Removed Unlock / Lock controls from the main menu. Lock Telegram wallet is available inside wallet screens.
- Wallet screens and password prompts report whether that chain's desktop manager vault is locked or unlocked.
- Removed the requirement to lock the manager before using Kaspa in Telegram. Desktop and Telegram maintain independent logical sessions through one serialized SDK storage owner. Wallet/account selections, credentials, lock state and payment review tokens remain separate. Opening in Telegram never unlocks the desktop.
- Retains the 0.9.27 balance fallback, copy buttons, separate chain permissions and optional tray background mode.

## Use

Stop the previous manager, extract the included Windows ZIP and run ZKasNodeManager.exe with its wallet-runtime folder. In Bot, enable the desired chain and remote lock/unlock if you want password prompts in Telegram. Open Wallets in Telegram. Local Bot-tab unlock buttons remain available when remote unlocking is disabled. Existing saved settings and application data are retained.

## Validation

Core Go tests and Windows-target vet passed; Windows amd64 GUI executable built with Go 1.26.8 and the embedded manifest. Official SDK offline tests cover two logical sessions on the same storage, independent locks and selections, rejected-password/filename isolation and address persistence. Adapter tests cover session-bound payment tokens, switching and replay rejection. Telegram tests cover prompted opening, resumption, wrong passwords, permission checks and main-menu cleanup.

Native Windows UI/tray behavior, live Telegram and live-node transactions have not been tested in this Linux environment.
