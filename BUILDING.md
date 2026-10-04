# Build on Windows

Use 64-bit Windows 10/11, Git, PowerShell and Go **1.26.8** (the pinned release toolchain). Clone this repository and open PowerShell in its root:

```powershell
./build.ps1
```

If local script policy blocks the script, review it before using an appropriate per-process policy; no system-wide policy change is required.

The script fetches the pinned Node.js 24.21.0 Windows executable from nodejs.org only if absent, verifies all six runtime files against the Go source manifest, runs tests and vet, generates the embedded Windows manifest, builds the GUI and test fixture, runs Windows lifecycle tests, executes published WASM recovery vectors and makes the complete runtime ZIP in `artifacts/`.

Source-controlled signer/WASM files are small prebuilt dependencies with pinned provenance. Node.exe and generated release files are deliberately excluded from git. First builds require network access to Go modules and nodejs.org. The release build does not use another copy of the app or files outside this checkout.

`dist/` is replaced on every build. Do not store anything personal there.

## Checks without Windows

```sh
go test ./internal/...
GOOS=windows GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags='-H windowsgui -s -w' -o ZKasNodeManager.exe ./cmd/zkas-manager
```

Cross-compilation does not run the UI, process lifecycle or firewall tests. Windows tests use a fake node in temporary directories; they do not broadcast payments or install startup entries.

## Dependency updates

Keep `go.mod`/`go.sum`, notices and tests consistent. Run `govulncheck` against the Windows target. GitHub CI pins the checker version and action commit hashes; Dependabot proposes module/action updates.

For runtime changes, review origin and license, update the hashes in `cmd/zkas-manager/wallet_runtime_hashes_windows.go`, and retest before releasing. Never just delete the hash check to accept a changed binary. To rebuild the viewing helper, see `view-tools/README.md`; that requires its pinned Rust toolchain. The upstream signing WASM is imported at a pinned commit, not rebuilt by the Go build.
