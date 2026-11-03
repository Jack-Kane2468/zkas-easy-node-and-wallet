# ZKas Node Manager 0.9.31-preview

- Telegram main menu → Notifications (or /notifications) offers independent on/off controls for block, unexpected node-stop, sync/RPC, mining-down, ZKas receipt, Kaspa receipt and daily-summary notifications.
- The paired owner can change these shared recipient settings. Existing private-chat authorization and button sender binding apply. Receive notifications still require the matching wallet-view permission.
- Save changes before applying them, mirror the manager's checkboxes and pick up settings during the next monitoring cycle (normally within 20 seconds). No bot restart is required, including when all notifications started disabled.
- Buttons set an explicit state, so an old On button cannot accidentally turn a setting off. Daily-summary schedule and mining thresholds remain configured in the manager.
- Rebaseline trackers when settings change; suppress results from a poll whose settings changed before delivery. Daily summary coverage restarts when daily summaries are toggled.

The manager's mining tabs, bridge selection and update behavior are unchanged from v0.9.30.
