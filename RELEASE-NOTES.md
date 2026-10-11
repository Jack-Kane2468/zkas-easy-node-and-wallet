# 0.9.10-preview — Kaspa Overview updates

- Added **Check manager updates** to Overview → Kaspa. Both Overview pages share the same manager update channel and preview-release setting.
- Added **Check node updates** to Overview → Kaspa. It checks the latest stable official kaspanet/rusty-kaspa Windows x64 release, shows installed/available versions, and verifies SHA-256 before stopping the node.
- Updates retain the Kaspa database, wallet vault, ports, sharing configuration and mining settings. Only the node executable is selected for replacement; bridge updates remain in Mining.
- A running Kaspa node restarts after updating. A stopped node stays stopped. Active Kaspa or merged mining is paused; start it again when Kaspa is ready. ZKas services and ZKas-only mining are unaffected.
- Repair avoids replacing an updated Kaspa node with an older installation pin.

Windows cross-build/static checks and portable tests pass. Native Windows upgrade/restart and live mining were not tested in the build environment.
