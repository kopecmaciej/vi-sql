#Requires -Version 5
$ErrorActionPreference = 'Stop'

$Repo = 'kopecmaciej/vi-sql'
$BinName = 'vi-sql.exe'

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'x86_64' }
    'ARM64' { 'arm64' }
    default { throw "unsupported arch: $env:PROCESSOR_ARCHITECTURE" }
}

$version = $env:VI_SQL_VERSION
if (-not $version) {
    Write-Host 'fetching latest release tag…'
    $version = (Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest").tag_name
    if (-not $version) { throw 'could not determine latest version' }
}

$asset = "vi-sql_Windows_$arch.zip"
$url = "https://github.com/$Repo/releases/download/$version/$asset"

$installDir = if ($env:VI_SQL_INSTALL_DIR) { $env:VI_SQL_INSTALL_DIR } else { "$env:LOCALAPPDATA\Programs\vi-sql" }
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

$tmp = Join-Path $env:TEMP "vi-sql-install-$(Get-Random)"
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
    Write-Host "downloading $asset ($version)…"
    $zip = Join-Path $tmp $asset
    Invoke-WebRequest -Uri $url -OutFile $zip

    Write-Host 'extracting…'
    Expand-Archive -Path $zip -DestinationPath $tmp -Force

    $bin = Get-ChildItem -Path $tmp -Recurse -Filter $BinName | Select-Object -First 1
    if (-not $bin) { throw "binary not found in archive" }

    Write-Host "installing to $installDir…"
    Copy-Item -Path $bin.FullName -Destination (Join-Path $installDir $BinName) -Force
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $installDir) {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$installDir", 'User')
    Write-Host "added $installDir to your user PATH — restart your terminal to pick it up"
}

Write-Host ''
Write-Host "installed: $installDir\$BinName ($version)"
