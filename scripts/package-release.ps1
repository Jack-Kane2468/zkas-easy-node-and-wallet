$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
Set-Location $root
if (-not (Test-Path dist/ZKasNodeManager.exe)) { throw 'Build the manager before packaging.' }
if (Test-Path dist/testnode.exe) { throw 'Test fixture must not be included in a public release.' }
& ./scripts/prepare-runtime.ps1
Remove-Item dist/wallet-runtime, dist/THIRD-PARTY-LICENSES -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item dist/SHA256SUMS.txt -Force -ErrorAction SilentlyContinue
Copy-Item -Recurse wallet-runtime dist/wallet-runtime
Get-ChildItem dist/wallet-runtime -Filter '*.download' | Remove-Item -Force
Get-ChildItem *.md | Copy-Item -Destination dist
Copy-Item LICENSE dist/LICENSE
Copy-Item DEPENDENCIES.json dist/DEPENDENCIES.json
Copy-Item view-tools/LICENSE dist/VIEW-TOOLS-LICENSE.txt
Copy-Item -Recurse view-tools/THIRD-PARTY-LICENSES dist/THIRD-PARTY-LICENSES
$hashes = Get-ChildItem dist -Recurse -File | Sort-Object FullName | ForEach-Object {
    $relative = $_.FullName.Substring((Join-Path $root 'dist').Length + 1).Replace('\','/')
    '{0}  {1}' -f (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant(), $relative
}
$hashes | Set-Content dist/SHA256SUMS.txt -Encoding ascii
New-Item -ItemType Directory -Force artifacts | Out-Null
$versionSource = Get-Content internal/node/node.go -Raw
$version = [regex]::Match($versionSource, 'const ManagerVersion = "([^"]+)"').Groups[1].Value
if (-not $version) { throw 'Cannot read manager version' }
$zip = Join-Path $root "artifacts/ZKasNodeManager-Windows-x64-$version.zip"
Compress-Archive -Path dist/* -DestinationPath $zip -Force
'{0}  {1}' -f (Get-FileHash $zip -Algorithm SHA256).Hash.ToLowerInvariant(), (Split-Path $zip -Leaf) | Set-Content artifacts/SHA256SUMS.txt -Encoding ascii
Write-Host "Complete release: $zip"
