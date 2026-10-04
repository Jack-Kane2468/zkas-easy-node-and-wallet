# Window and existing-installation compatibility

Close only the previous manager window, extract this complete folder and open ZKasNodeManager.exe. No install/repair action is needed to open the new UI. Existing node/wallet/mining services can remain running. Opening the manager reads settings and checks status; it does not change listeners or reset data.

The window starts at 680 × 520, clamped to the monitor work area. It can shrink to a small minimum, with scrolling content. Use Compact / restore, Window > Compact / restore, or Escape to restore it. Overview, Wallet, Mining, Sharing, Settings and Logs are separate tabs.

All copy buttons use Windows clipboard ownership correctly, retry brief contention and display success/errors. The old app-specific copy button now copies a plain wallet API URL.

Changing peer exposure requires stopped services. Adding HTTPS/onion sharing starts a separate host without restarting the node/wallet. Enabling mining statistics for a previously running old bridge requires only a mining restart.

Existing data paths and node/wallet/mining host identities are preserved. New networking settings default to disabled. A separate sharing host and configuration hold optional remote-access settings. Start menu shortcuts still point to the installed manager until it is replaced with services stopped.

Dropdowns and single-line numeric fields now ignore mouse-wheel changes. Scrolling over them is forwarded to their parent instead; click an option or type a value intentionally.
