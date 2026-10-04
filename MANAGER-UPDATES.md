# Manager updates

Overview → Check manager updates checks releases from https://github.com/Jack-Kane2468/zkas-easy-node-and-wallet. Keep Include preview releases checked for the preview series. Check node updates remains a separate button.

The manager displays the new version and release notes for confirmation, verifies the download and packaged files, then restarts its window. The wallet vault locks. Running node, wallet backend, mining and sharing processes are left running; existing settings and wallet/chain data remain in their existing locations.

New managers are installed into separate manager-versions folders under the existing application data folder. Old files are retained. If the new window fails to initialize, the helper attempts to reopen the previous executable. Startup recovery does not guarantee that every later feature will work.

## First upgrade from 0.8.3

Extract the complete Windows ZIP into a new folder. Close the old manager window and run the new ZKasNodeManager.exe. Services can stay running when opening the new copy. Use this copy for the new button.

To replace the old installed Start menu copy immediately, stop services, select Settings → Install / repair components in this version, then start services. This is a one-time manual upgrade because 0.8.3 lacks the updater. Later successful in-app updates refresh installed shortcuts automatically.

## Publishing

Update ManagerVersion in internal/node/node.go and the Windows manifest version. Run build.ps1 on Windows or use the included GitHub workflow. Complete the Windows smoke test below.

Use the matching release tag (for example v0.8.5-preview). Attach BOTH artifacts/ZKasNodeManager-Windows-x64-0.8.5-preview.zip and artifacts/SHA256SUMS.txt to that release, then publish. Do not rename the Windows ZIP. Drafts are ignored, and previews need the checkbox enabled.

The checksum file attached to the GitHub release hashes the Windows ZIP. The checksum file inside the ZIP hashes individual files. GitHub's automatic source archives are not runnable update packages.

Checksums catch damage or mismatched files. They do not protect against compromise of the GitHub publisher account. These releases are unsigned and unaudited.

## Windows smoke test before publishing

- Open the new manager against an existing installation and confirm service status and wallet history.
- Check for updates with no newer release available.
- Test a reviewed higher-version preview with both matching assets attached.
- Confirm the new window opens, the vault locks, services stay running, and installed shortcuts/autostart use the new version.
- Confirm interrupted or invalid downloads leave the previous manager usable.
- In a disposable Windows profile, test failed-startup recovery and uninstall after an update; retained wallet and chain data should remain.
