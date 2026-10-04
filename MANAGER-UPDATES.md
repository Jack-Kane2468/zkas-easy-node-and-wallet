# Manager updates

**Overview → Check manager updates** checks this project's published GitHub releases for a version newer than the running manager. Draft releases are ignored. **Include preview releases** allows prereleases and versions with a preview suffix.

The new version and release notes appear before confirmation. The download and packaged files are verified before the manager window restarts. The wallet vault locks. Running node, wallet backend, mining and sharing processes continue; existing settings and wallet/chain data stay in their existing locations.

Each updated manager uses a separate application folder. Previous files are retained, and the updater attempts to reopen the previous manager if the new window fails to initialize. This recovery covers initial startup, not every later application operation. Installed shortcuts are refreshed after a successful update.

**Check node updates** is separate and checks the upstream node software.

## Manual updates

Older versions without the update button require a complete Windows release download. Extract it into a new folder, close the previous manager window and run the new EXE. Opening the new window reuses the existing installation without stopping services.

A Windows Start menu shortcut may still point to an older installed copy. To replace that copy, stop services, open **Install / setup**, use **Install / repair components**, then start services again. This step is optional when running the extracted EXE directly.

## Download errors

Missing release assets, interrupted downloads or checksum mismatches leave the current manager unchanged. GitHub rate limits or connectivity problems may require retrying later.

Checksums detect damaged or mismatched files; they are not publisher signatures. Release integrity also depends on the security of the project's GitHub account.
