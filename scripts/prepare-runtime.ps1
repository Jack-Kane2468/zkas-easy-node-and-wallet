$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$runtime = Join-Path $root 'wallet-runtime'
$manifest = Get-Content (Join-Path $root 'cmd/zkas-manager/wallet_runtime_hashes_windows.go') -Raw
$matches = [regex]::Matches($manifest, '"([^"]+)":\s*"([0-9a-f]{64})"')
if ($matches.Count -ne 6) { throw 'Unexpected runtime manifest; review before downloading anything.' }
$nodePath = Join-Path $runtime 'node.exe'
$nodeHash = ($matches | Where-Object { $_.Groups[1].Value -eq 'node.exe' }).Groups[2].Value
if (-not (Test-Path $nodePath)) {
    $temp = "$nodePath.download"
    try {
        Invoke-WebRequest 'https://nodejs.org/dist/v24.21.0/win-x64/node.exe' -OutFile $temp
        if ((Get-FileHash $temp -Algorithm SHA256).Hash.ToLowerInvariant() -ne $nodeHash) { throw 'Node download checksum mismatch.' }
        Move-Item $temp $nodePath
    } finally { Remove-Item $temp -Force -ErrorAction SilentlyContinue }
}
foreach ($match in $matches) {
    $file = Join-Path $runtime $match.Groups[1].Value
    if (-not (Test-Path $file -PathType Leaf)) { throw "Missing runtime file: $file" }
    if ((Get-FileHash $file -Algorithm SHA256).Hash.ToLowerInvariant() -ne $match.Groups[2].Value) { throw "Runtime checksum mismatch: $file" }
}
Write-Host 'All six wallet runtime files verified.'
