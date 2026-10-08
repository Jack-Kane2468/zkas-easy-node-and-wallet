# 0.9.1-preview — Mining modes and Kaspa control states

- Shared mining now offers Merged, Kaspa-only and ZKas-only. ZKas-only does not require a Kaspa node or Kaspa address.
- Kaspa setup disappears after installation. Start/Stop reflect the running state; repair and conflicting actions disable while unavailable.
- Install/start/stop/uninstall operations immediately disable conflicting controls and report a specific completion message.
- Node status uses color and an activity indicator, with PID, version, address-index status, last reported sync message and check time. The activity bar is not a claimed completion percentage.
- Mining controls follow the selected mode, installation, required node sync and payout fields. Irrelevant payout fields hide. Dashboard access requires a running bridge.
- A Kaspa Wallet tab installs and opens the checksum-verified upstream wallet console, with local-connection instructions and wallet commands. This is terminal-based; graphical create/import/send screens like the ZKas wallet are not implemented.
- Existing chain data, running nodes, ZKas wallets and bounded log rendering are preserved. No reinstall is required merely to use the new manager controls.

Windows cross-build/static checks and portable tests passed. Native Windows GUI interactions and live mining/payments have not been tested in this environment. Close the upstream wallet console before uninstalling its program files.
