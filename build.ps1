<#
.SYNOPSIS
    Builds Lumen: SPA -> embedded assets -> Windows GUI binary -> installer.

.DESCRIPTION
    Three stages, any of which can be skipped:
      1. npm build   — compiles the SolidJS app into internal/server/web/dist,
                       which is what //go:embed picks up. Skipping this and then
                       building Go embeds whatever is already there.
      2. go build    — with -H windowsgui so no console window appears, and the
                       version stamped into the binary via -ldflags.
      3. ISCC        — Inno Setup, producing dist\LumenSetup.exe.

.PARAMETER Version
    Version string to stamp. Defaults to `git describe`, then 0.0.0-dev.

.PARAMETER Installer
    Also build the installer. Requires Inno Setup 6.

.PARAMETER SkipWeb
    Skip the npm build and reuse the existing dist output.

.PARAMETER Console
    Build WITHOUT -H windowsgui, so the app keeps a console window. Useful when
    debugging startup problems that happen before logging is up.

.EXAMPLE
    .\build.ps1
    .\build.ps1 -Version 1.0.0 -Installer
    .\build.ps1 -SkipWeb -Console
#>
[CmdletBinding()]
param(
    [string] $Version,
    [switch] $Installer,
    [switch] $SkipWeb,
    [switch] $Console
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
Set-Location $root

function Write-Stage($msg) { Write-Host "`n=== $msg ===" -ForegroundColor Cyan }
function Write-Ok($msg)    { Write-Host "  OK  $msg" -ForegroundColor Green }
function Write-Info($msg)  { Write-Host "      $msg" -ForegroundColor DarkGray }

# ---------------------------------------------------------------- version ----
if (-not $Version) {
    $described = (& git describe --tags --always --dirty 2>$null)
    if ($LASTEXITCODE -eq 0 -and $described) {
        $Version = $described -replace '^v', ''
    } else {
        $Version = '0.0.0-dev'
    }
}
# Windows VERSIONINFO needs a strict 4-part numeric version. Derive one from the
# leading x.y.z of whatever we were given; suffixes like -rc1 or -dirty are kept
# for the string version but stripped for the numeric one.
if ($Version -match '^(\d+)\.(\d+)\.(\d+)') {
    $verMajor = [int]$Matches[1]; $verMinor = [int]$Matches[2]; $verPatch = [int]$Matches[3]
} else {
    $verMajor = 0; $verMinor = 0; $verPatch = 0
}
Write-Stage "Lumen $Version  ($verMajor.$verMinor.$verPatch.0)"

# ------------------------------------------------------------------- web -----
if (-not $SkipWeb) {
    Write-Stage 'Building SPA'
    Push-Location "$root\web"
    try {
        if (-not (Test-Path 'node_modules')) {
            Write-Info 'node_modules missing — running npm ci'
            & npm ci
            if ($LASTEXITCODE -ne 0) { throw "npm ci failed ($LASTEXITCODE)" }
        }
        & npm run build
        if ($LASTEXITCODE -ne 0) { throw "npm run build failed ($LASTEXITCODE)" }
    } finally { Pop-Location }

    $index = "$root\internal\server\web\dist\index.html"
    if (-not (Test-Path $index)) { throw "SPA build produced no $index" }
    $distSize = (Get-ChildItem "$root\internal\server\web\dist" -Recurse -File | Measure-Object -Property Length -Sum).Sum
    Write-Ok ("SPA built — {0:N1} MB of embedded assets" -f ($distSize / 1MB))
} else {
    Write-Info 'Skipping SPA build (-SkipWeb)'
}

# --------------------------------------------------------- version resource --
Write-Stage 'Stamping Windows version resource'
$goversioninfo = Get-Command goversioninfo -ErrorAction SilentlyContinue
if (-not $goversioninfo) {
    $candidate = Join-Path (& go env GOPATH) 'bin\goversioninfo.exe'
    if (Test-Path $candidate) { $goversioninfo = $candidate }
}
if ($goversioninfo) {
    $exe = if ($goversioninfo -is [string]) { $goversioninfo } else { $goversioninfo.Source }
    & $exe -o "$root\cmd\lumen\resource_windows_amd64.syso" `
        -platform-specific=false `
        -ver-major $verMajor -ver-minor $verMinor -ver-patch $verPatch -ver-build 0 `
        -product-ver-major $verMajor -product-ver-minor $verMinor -product-ver-patch $verPatch -product-ver-build 0 `
        -file-version $Version -product-version $Version `
        -icon "$root\assets\lumen.ico" `
        "$root\assets\versioninfo.json"
    if ($LASTEXITCODE -ne 0) { throw "goversioninfo failed ($LASTEXITCODE)" }
    Write-Ok 'Icon + version resource stamped'
} else {
    Write-Warning 'goversioninfo not found — reusing the existing .syso (icon/version may be stale)'
    Write-Info  'Install with: go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest'
}

# -------------------------------------------------------------------- go -----
Write-Stage 'Building lumen.exe'
$ldflags = "-s -w -X main.version=$Version"
if (-not $Console) { $ldflags = "-H windowsgui $ldflags" }

$out = "$root\lumen.exe"
if (Test-Path $out) {
    # A running instance holds a lock on the exe; a clear message beats Go's.
    try { Remove-Item $out -Force -ErrorAction Stop }
    catch { throw "Cannot overwrite lumen.exe — is Lumen still running? Quit it from the tray and retry." }
}

& go build -trimpath -ldflags $ldflags -o $out ./cmd/lumen
if ($LASTEXITCODE -ne 0) { throw "go build failed ($LASTEXITCODE)" }

$sz = (Get-Item $out).Length
Write-Ok ("lumen.exe built — {0:N1} MB{1}" -f ($sz / 1MB), $(if ($Console) { ' (console build)' } else { '' }))

# ------------------------------------------------------------- installer -----
if ($Installer) {
    Write-Stage 'Building installer'
    $iscc = $null
    foreach ($p in @(
        "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
        "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
        "$env:ProgramFiles\Inno Setup 6\ISCC.exe"
    )) { if (Test-Path $p) { $iscc = $p; break } }
    if (-not $iscc) { $iscc = (Get-Command ISCC.exe -ErrorAction SilentlyContinue)?.Source }
    if (-not $iscc) {
        throw "Inno Setup 6 not found. Install it with: winget install JRSoftware.InnoSetup"
    }
    Write-Info "using $iscc"

    New-Item -ItemType Directory -Force -Path "$root\dist" | Out-Null
    & $iscc "/DMyAppVersion=$Version" "$root\installer\lumen.iss"
    if ($LASTEXITCODE -ne 0) { throw "ISCC failed ($LASTEXITCODE)" }

    $setup = "$root\dist\LumenSetup.exe"
    if (-not (Test-Path $setup)) { throw "ISCC reported success but $setup is missing" }
    Write-Ok ("installer built — dist\LumenSetup.exe, {0:N1} MB" -f ((Get-Item $setup).Length / 1MB))
}

Write-Host "`nDone." -ForegroundColor Green
