# Logs

Choose Node, Wallet backend, Mining bridge or Tor from the Logs tab.

The view follows new entries while positioned at the bottom. Scrolling up or selecting text holds the current snapshot so incoming entries do not move it. Scroll back to the bottom and clear any selection to resume, or click **Latest** to immediately catch up. **Pause** freezes updates until unchecked; **Latest** also clears Pause.

Timestamps are muted. Errors are red, warnings amber, synchronization progress cyan and informational entries green. Original severity labels remain visible. Coloring is a display aid and does not change the underlying log files.

The viewer shows a bounded tail of up to 128 KiB. A held snapshot stays visible even when newer entries exceed that limit. Use **Open log folder** for older history or the full files. Switching the selected log opens its most recent entries.

## Resource use

New entries are appended and colored incrementally. The viewer retains a bounded recent-log window and has no undo history. Unchanged files are not reread, and hidden, paused or held log views do not read new log content. Regular status polling pauses while minimized. A wallet keepalive remains every 30 seconds to retain active-wallet behavior; node and wallet services run independently.
