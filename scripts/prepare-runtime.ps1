$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$runtime = Join-Path $root 'wallet-runtime'
$manifest = Get-Content (Join-Path $root 'cmd/zkas-manager/wallet_runtime_hashes_windows.go') -Raw
$matches = [regex]::Matches($manifest, '"([^"]+)":\s*"([0-9a-f]{64})"')
if ($matches.Count -ne 10) { throw 'Unexpected runtime manifest; review before downloading anything.' }
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
# Fetch the official SDK when building from the source-only tree.
if (-not (Test-Path (Join-Path $runtime 'kaspa.cjs')) -or -not (Test-Path (Join-Path $runtime 'kaspa_bg.wasm'))) {
    $sdkZip = Join-Path $runtime 'kaspa-sdk.zip.download'
    try {
        Invoke-WebRequest 'https://github.com/kaspanet/rusty-kaspa/releases/download/v2.1.0/kaspa-wasm32-sdk-v2.1.0.zip' -OutFile $sdkZip
        if ((Get-FileHash $sdkZip -Algorithm SHA256).Hash.ToLowerInvariant() -ne 'ba674e109ff5dd8bedc4dc2ee8a5ecdf4b600b1178a541d77888ec58310b6124') { throw 'Kaspa SDK checksum mismatch.' }
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        $archive = [IO.Compression.ZipFile]::OpenRead($sdkZip)
        try {
            foreach ($pair in @(@('kaspa.js','kaspa.cjs'), @('kaspa_bg.wasm','kaspa_bg.wasm'))) {
                $entry = $archive.GetEntry('kaspa-wasm32-sdk/nodejs/kaspa/' + $pair[0])
                if (-not $entry -or $entry.Length -gt 20971520) { throw 'Unexpected SDK entry.' }
                $target = Join-Path $runtime $pair[1]
                [IO.Compression.ZipFileExtensions]::ExtractToFile($entry, "$target.download", $true)
                Move-Item "$target.download" $target -Force
            }
        } finally { $archive.Dispose() }
    } finally { Remove-Item $sdkZip -Force -ErrorAction SilentlyContinue }
}
foreach ($match in $matches) {
    $file = Join-Path $runtime $match.Groups[1].Value
    if (-not (Test-Path $file -PathType Leaf)) { throw "Missing runtime file: $file" }
    if ((Get-FileHash $file -Algorithm SHA256).Hash.ToLowerInvariant() -ne $match.Groups[2].Value) { throw "Runtime checksum mismatch: $file" }
}
Write-Host 'All nine wallet runtime files verified.'
