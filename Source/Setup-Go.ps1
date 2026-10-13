$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$goModPath = Join-Path $PSScriptRoot 'go.mod'
if (-not (Test-Path -LiteralPath $goModPath -PathType Leaf)) {
    throw "go.mod wurde nicht gefunden: $goModPath"
}

$goDirective = Get-Content -LiteralPath $goModPath -Raw
if ($goDirective -notmatch '(?m)^go\s+(?<version>\d+\.\d+(?:\.\d+)?)\s*$') {
    throw "Eine gueltige Go-Version konnte in go.mod nicht gelesen werden: $goModPath"
}

$minimumGoVersion = [version]$Matches.version
if ($minimumGoVersion.Build -lt 0) {
    $minimumGoVersion = [version]("$($Matches.version).0")
}
$packageId = 'GoLang.Go'

function Find-GoExecutable {
    $candidates = @()
    $command = Get-Command go.exe -ErrorAction SilentlyContinue
    if ($command) {
        $candidates += $command.Source
    }

    $candidates += @(
        (Join-Path $env:ProgramFiles 'Go\bin\go.exe'),
        (Join-Path $env:LOCALAPPDATA 'Programs\Go\bin\go.exe')
    )

    foreach ($candidate in $candidates | Select-Object -Unique) {
        if ($candidate -and (Test-Path -LiteralPath $candidate -PathType Leaf)) {
            return $candidate
        }
    }

    return $null
}

function Get-GoInstallation {
    $executable = Find-GoExecutable
    if (-not $executable) {
        return $null
    }

    $versionOutput = & $executable version
    if ($LASTEXITCODE -ne 0 -or $versionOutput -notmatch '\bgo(?<version>\d+\.\d+(?:\.\d+)?)\s+windows/amd64\b') {
        throw "Go wurde gefunden, meldet aber keine gueltige Windows/amd64-Version: $versionOutput"
    }

    return [pscustomobject]@{
        Path = $executable
        Version = [version]$Matches.version
    }
}

$architecture = $env:PROCESSOR_ARCHITECTURE
if ($env:PROCESSOR_ARCHITEW6432) {
    $architecture = $env:PROCESSOR_ARCHITEW6432
}

if ($architecture -ne 'AMD64') {
    throw 'Dieses Projekt benoetigt einen Windows-x64-PC (AMD64).'
}

$go = Get-GoInstallation
if (-not $go) {
    $winget = Get-Command winget.exe -ErrorAction SilentlyContinue
    if (-not $winget) {
        throw 'winget wurde nicht gefunden. Installiere zuerst "App-Installer" aus dem Microsoft Store und starte dieses Script erneut.'
    }

    Write-Host 'Installiere Go ueber den Windows-Paketmanager (winget)...' -ForegroundColor Cyan
    & $winget.Source install --id $packageId --exact --source winget --accept-package-agreements --accept-source-agreements --silent
    if ($LASTEXITCODE -ne 0) {
        throw "Go-Installation fehlgeschlagen (winget Exit-Code $LASTEXITCODE)."
    }

    $go = Get-GoInstallation
} elseif ($go.Version -lt $minimumGoVersion) {
    $winget = Get-Command winget.exe -ErrorAction SilentlyContinue
    if (-not $winget) {
        throw "Go $($go.Version) ist zu alt; mindestens Go $minimumGoVersion ist erforderlich. winget wurde nicht gefunden."
    }

    Write-Host "Aktualisiere Go $($go.Version) auf mindestens $minimumGoVersion..." -ForegroundColor Cyan
    & $winget.Source upgrade --id $packageId --exact --source winget --accept-package-agreements --accept-source-agreements --silent
    if ($LASTEXITCODE -ne 0) {
        throw "Go-Upgrade fehlgeschlagen (winget Exit-Code $LASTEXITCODE)."
    }

    $go = Get-GoInstallation
}

if (-not $go -or $go.Version -lt $minimumGoVersion) {
    throw "Go $minimumGoVersion oder neuer ist nach der Installation nicht verfuegbar. Schliesse das Terminal, oeffne es erneut und starte das Script noch einmal."
}

$goBin = Split-Path -Parent $go.Path
if ($env:PATH -notlike "*$goBin*") {
    $env:PATH = "$goBin;$env:PATH"
}

Write-Host ''
Write-Host "Go $($go.Version) ist bereit: $($go.Path)" -ForegroundColor Green
Write-Host 'Go-Modul und Windows-x64-Toolchain geprueft.' -ForegroundColor Green
Write-Host 'Zum Bauen der Anwendung danach Build-Local.cmd in diesem Ordner ausfuehren.' -ForegroundColor Cyan
Write-Host 'ViGEmBus und ViGEmClient.dll werden nicht installiert oder veraendert.'
Write-Host ''
Read-Host 'Enter zum Schliessen'
