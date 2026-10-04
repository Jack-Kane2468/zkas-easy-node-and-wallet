# Put this project on GitHub

The `Repository` folder in the sharing kit is the repository root. Upload its contents, not the outer kit or just its ZIP. The supplied Windows ZIP is for release attachments, not source control.

1. Create an **empty** GitHub repository named `zkas-node-manager` (or your preferred name). Do not add an extra README or license there.
2. Open a terminal inside `Repository`. With Git installed, run the following, replacing YOUR_ACCOUNT with your GitHub username and adjusting the repository name if needed:

```sh
git init -b main
git add .
git commit -m "Prepare ZKas Node Manager community preview"
git remote add origin https://github.com/YOUR_ACCOUNT/zkas-node-manager.git
git push -u origin main
```

Git may first ask you to configure your commit name/email and sign in to GitHub. Use your own identity and GitHub's normal authentication; do not paste access tokens into source files.

3. Open **Actions** and wait for the Windows checks. A source upload alone is not proof of a working Windows build.
4. Enable **Settings → Code security → Private vulnerability reporting** (wording may vary) and review branch protection/dependency alerts.
5. Follow RELEASE-CHECKLIST.md. When ready for a draft release:

```sh
git tag v0.8.3-preview
git push origin v0.8.3-preview
```

CI creates a draft prerelease only after checks succeed. Review it before publishing. You may also attach the complete Windows ZIP from this kit manually, with its checksum and validation notes; it was cross-compiled and has not passed a native Windows run here.

Never upload `%LOCALAPPDATA%\ZKasNodeManager`. That is your private live installation and may contain wallet secrets and tokens. This kit's Repository folder is the intended public source.
