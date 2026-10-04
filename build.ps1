$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
# Only these generated directories are replaced. Application data is elsewhere.
Remove-Item (Join-Path $PSScriptRoot 'dist') -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force dist | Out-Null
& ./scripts/prepare-runtime.ps1
# Regenerate the executable's manifest from reviewed source.
go run github.com/akavel/rsrc@v0.10.2 -manifest cmd/zkas-manager/app.manifest -arch amd64 -o cmd/zkas-manager/rsrc_windows_amd64.syso
if ($LASTEXITCODE -ne 0) { throw 'Manifest generation failed' }
go test ./internal/...
if ($LASTEXITCODE -ne 0) { throw 'Core tests failed' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Vet failed' }
go build -buildvcs=false -trimpath -ldflags '-H windowsgui -s -w' -o dist/ZKasNodeManager.exe ./cmd/zkas-manager
if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
go build -buildvcs=false -tags integrationfixture -o dist/testnode.exe ./cmd/testnode
if ($LASTEXITCODE -ne 0) { throw 'Test fixture build failed' }
$env:ZKAS_MANAGER_TEST_EXE = Join-Path $PSScriptRoot 'dist/ZKasNodeManager.exe'
$env:ZKAS_FAKE_NODE_EXE = Join-Path $PSScriptRoot 'dist/testnode.exe'
try {
    go test -count=1 -timeout 5m -v ./cmd/zkas-manager
    if ($LASTEXITCODE -ne 0) { throw 'Windows lifecycle tests failed' }
} finally {
    Remove-Item Env:ZKAS_MANAGER_TEST_EXE, Env:ZKAS_FAKE_NODE_EXE -ErrorAction SilentlyContinue
    Remove-Item dist/testnode.exe -ErrorAction SilentlyContinue
}
& ./wallet-runtime/node.exe ./view-tools/check-vectors.mjs
if ($LASTEXITCODE -ne 0) { throw 'WASM recovery vectors failed' }
& ./scripts/package-release.ps1
