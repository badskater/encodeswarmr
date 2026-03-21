<#
.SYNOPSIS
    Installs the Distributed Encoder agent on Windows Server 2019 / 2022.

.DESCRIPTION
    Creates the directory structure, downloads (or uses a local) agent binary,
    generates agent.yaml from a template, verifies encoding tool paths,
    installs and starts the Windows Service.

.PARAMETER ControllerAddress
    Controller gRPC address including port. Example: encoder.example.com:9443

.PARAMETER AgentHostname
    Name for this agent (used in logs, UI, and cert file names).
    Defaults to the machine's computer name ($env:COMPUTERNAME).

.PARAMETER InstallDir
    Root directory for agent data (work files, logs, certs, offline DB).
    Default: C:\DistEncoder

.PARAMETER CertDir
    Directory that already contains ca.crt, <AgentHostname>.crt, <AgentHostname>.key.
    Default: <InstallDir>\certs
    Files are copied from here into the install directory if needed.

.PARAMETER AgentBinary
    Full path to an already-downloaded distencoder-agent.exe.
    If not provided, the binary is downloaded from GitHub releases.

.PARAMETER Version
    Release version to download (e.g. "1.0.0" — without the "v" prefix).
    Required when AgentBinary is not provided.

.EXAMPLE
    # Interactive install (prompts for missing values)
    .\install-agent.ps1

.EXAMPLE
    # Non-interactive install with local binary
    .\install-agent.ps1 `
        -ControllerAddress "encoder.example.com:9443" `
        -AgentHostname "ENCODE-01" `
        -AgentBinary "C:\Downloads\distencoder-agent.exe" `
        -CertDir "C:\Downloads\certs"

.EXAMPLE
    # Download binary from GitHub releases
    .\install-agent.ps1 `
        -ControllerAddress "10.0.0.5:9443" `
        -Version "1.0.0"
#>

[CmdletBinding()]
param(
    [string]$ControllerAddress,
    [string]$AgentHostname = $env:COMPUTERNAME,
    [string]$InstallDir    = 'C:\DistEncoder',
    [string]$CertDir       = '',
    [string]$AgentBinary   = '',
    [string]$Version       = ''
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ── Helpers ───────────────────────────────────────────────────────────────────
function Write-Step  ([string]$n, [string]$msg) { Write-Host "`n[$n] $msg" -ForegroundColor Cyan }
function Write-OK    ([string]$msg) { Write-Host "    OK      $msg" -ForegroundColor Green }
function Write-Warn  ([string]$msg) { Write-Host "    WARN    $msg" -ForegroundColor Yellow }

function Read-Required {
    param([string]$Prompt, [string]$Default = '')
    if ($Default) {
        $val = Read-Host "$Prompt [$Default]"
        return if ($val) { $val } else { $Default }
    }
    while ($true) {
        $val = Read-Host $Prompt
        if ($val) { return $val }
        Write-Host "    Value is required." -ForegroundColor Red
    }
}

# ── Admin check ───────────────────────────────────────────────────────────────
$principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'This script must be run as Administrator. Right-click PowerShell → Run as Administrator.'
}

# ── Collect missing parameters interactively ──────────────────────────────────
if (-not $ControllerAddress) {
    $ControllerAddress = Read-Required 'Controller address (e.g. encoder.example.com:9443)'
}
if (-not $AgentHostname) {
    $AgentHostname = Read-Required 'Agent hostname' $env:COMPUTERNAME
}
if (-not $CertDir) {
    $CertDir = Join-Path $InstallDir 'certs'
}

Write-Host ''
Write-Host '============================================================' -ForegroundColor Cyan
Write-Host '  Distributed Encoder Agent Installer' -ForegroundColor Cyan
Write-Host '============================================================' -ForegroundColor Cyan
Write-Host "  Agent hostname  : $AgentHostname"
Write-Host "  Controller      : $ControllerAddress"
Write-Host "  Install dir     : $InstallDir"
Write-Host ''

# ── Step 1: Create directory structure ───────────────────────────────────────
Write-Step '1/7' "Creating directory structure at $InstallDir"
$dirs = @(
    $InstallDir,
    (Join-Path $InstallDir 'work'),
    (Join-Path $InstallDir 'logs'),
    (Join-Path $InstallDir 'certs')
)
foreach ($d in $dirs) {
    if (-not (Test-Path $d)) {
        New-Item -ItemType Directory -Path $d -Force | Out-Null
        Write-OK "Created $d"
    } else {
        Write-OK "Exists  $d"
    }
}

# ── Step 2: Resolve agent binary ─────────────────────────────────────────────
Write-Step '2/7' 'Resolving agent binary'
$BinaryDest = Join-Path $InstallDir 'distencoder-agent.exe'

if ($AgentBinary) {
    if (-not (Test-Path $AgentBinary)) {
        throw "AgentBinary not found: $AgentBinary"
    }
    Copy-Item -Path $AgentBinary -Destination $BinaryDest -Force
    Write-OK "Copied from $AgentBinary"
} else {
    if (-not $Version) {
        $Version = Read-Required 'Version to download from GitHub (e.g. 1.0.0 — without v prefix)'
    }
    $url = "https://github.com/badskater/distributed-encoder/releases/download/v${Version}/agent-windows-amd64.exe"
    Write-OK "Downloading from $url"
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $url -OutFile $BinaryDest -UseBasicParsing
    Write-OK "Saved to $BinaryDest"
}

# ── Step 3: Copy certificates ─────────────────────────────────────────────────
Write-Step '3/7' 'Checking certificate files'
$destCertDir = Join-Path $InstallDir 'certs'
$caFile      = Join-Path $destCertDir 'ca.crt'
$certFile    = Join-Path $destCertDir "${AgentHostname}.crt"
$keyFile     = Join-Path $destCertDir "${AgentHostname}.key"

# If CertDir differs from the destination, try to copy files
if ((Resolve-Path $CertDir -ErrorAction SilentlyContinue) -ne $destCertDir) {
    $srcFiles = @(
        (Join-Path $CertDir 'ca.crt'),
        (Join-Path $CertDir "${AgentHostname}.crt"),
        (Join-Path $CertDir "${AgentHostname}.key")
    )
    foreach ($f in $srcFiles) {
        if (Test-Path $f) {
            Copy-Item $f $destCertDir -Force
            Write-OK "Copied $(Split-Path $f -Leaf)"
        }
    }
}

$certsMissing = 0
foreach ($f in @($caFile, $certFile, $keyFile)) {
    if (Test-Path $f) {
        Write-OK "Found   $(Split-Path $f -Leaf)"
    } else {
        Write-Warn "Missing $(Split-Path $f -Leaf)"
        $certsMissing++
    }
}
if ($certsMissing -gt 0) {
    Write-Warn "$certsMissing cert file(s) missing. Copy them before starting the service."
    Write-Warn "Expected in: $destCertDir"
}

# ── Step 4: Write agent.yaml ──────────────────────────────────────────────────
Write-Step '4/7' 'Writing agent.yaml'
$ConfigDir  = 'C:\ProgramData\distributed-encoder'
$ConfigPath = Join-Path $ConfigDir 'agent.yaml'

if (-not (Test-Path $ConfigDir)) {
    New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
}

# Escape backslashes for YAML double-quoted strings
function ConvertTo-YamlPath ([string]$p) { $p.Replace('\', '\\') }

$workDir   = ConvertTo-YamlPath (Join-Path $InstallDir 'work')
$logDir    = ConvertTo-YamlPath (Join-Path $InstallDir 'logs')
$offlineDb = ConvertTo-YamlPath (Join-Path $InstallDir 'offline.db')
$yamlCert  = ConvertTo-YamlPath $certFile
$yamlKey   = ConvertTo-YamlPath $keyFile
$yamlCA    = ConvertTo-YamlPath $caFile

$configYaml = @"
controller:
  address: "$ControllerAddress"
  tls:
    cert: "$yamlCert"
    key:  "$yamlKey"
    ca:   "$yamlCA"
  reconnect:
    initial_delay: 5s
    max_delay: 5m
    multiplier: 2.0

agent:
  hostname: "$AgentHostname"
  work_dir:   "$workDir"
  log_dir:    "$logDir"
  offline_db: "$offlineDb"
  heartbeat_interval: 30s
  poll_interval: 10s
  cleanup_on_success: true
  keep_failed_jobs: 10

tools:
  ffmpeg:   "C:\\Tools\\ffmpeg\\ffmpeg.exe"
  ffprobe:  "C:\\Tools\\ffmpeg\\ffprobe.exe"
  x265:     "C:\\Tools\\x265\\x265.exe"
  x264:     "C:\\Tools\\x264\\x264.exe"
  svt_av1:  ""
  avs_pipe: "C:\\Program Files\\AviSynth+\\avs2pipemod.exe"
  vspipe:   "C:\\Program Files\\VapourSynth\\vspipe.exe"

gpu:
  enabled: true
  vendor: ""
  max_vram_mb: 0
  monitor_interval: 5s

allowed_shares: []

logging:
  level: info
  format: json
  max_size_mb: 100
  max_backups: 5
  compress: true
  stream_buffer_size: 1000
  stream_flush_interval: 1s

vnc:
  enabled: false
  port: 5900
"@

Set-Content -Path $ConfigPath -Value $configYaml -Encoding UTF8
Write-OK "Written to $ConfigPath"

# ── Step 5: Verify encoding tools ────────────────────────────────────────────
Write-Step '5/7' 'Verifying encoding tools'
$tools = [ordered]@{
    'FFmpeg'      = 'C:\Tools\ffmpeg\ffmpeg.exe'
    'FFprobe'     = 'C:\Tools\ffmpeg\ffprobe.exe'
    'x265'        = 'C:\Tools\x265\x265.exe'
    'x264'        = 'C:\Tools\x264\x264.exe'
    'AviSynth+'   = 'C:\Program Files\AviSynth+\avs2pipemod.exe'
    'VapourSynth' = 'C:\Program Files\VapourSynth\vspipe.exe'
}

Write-Host ''
Write-Host ('  {0,-14} {1,-50} {2}' -f 'Tool', 'Expected Path', 'Status')
Write-Host ('  ' + ('-' * 72))

$missing = 0
foreach ($entry in $tools.GetEnumerator()) {
    $found = Test-Path $entry.Value
    if ($found) {
        Write-Host ('  {0,-14} {1,-50} {2}' -f $entry.Key, $entry.Value, 'FOUND  ') -ForegroundColor Green
    } else {
        Write-Host ('  {0,-14} {1,-50} {2}' -f $entry.Key, $entry.Value, 'MISSING') -ForegroundColor Yellow
        $missing++
    }
}
Write-Host ''

if ($missing -gt 0) {
    Write-Warn "$missing tool(s) not found at expected paths."
    Write-Warn 'The agent will start but encoding jobs requiring missing tools will fail.'
    Write-Warn "Update tool paths in $ConfigPath after installing the tools."
    Write-Warn 'See DEPLOYMENT.md §1.4 for download links and installation notes.'
} else {
    Write-OK 'All tools found.'
}

# ── Step 6: Install Windows Service ──────────────────────────────────────────
Write-Step '6/7' 'Installing Windows Service'
$serviceName = 'distributed-encoder-agent'

$existingSvc = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($existingSvc) {
    Write-Warn "Service '$serviceName' already exists — removing it first."
    if ($existingSvc.Status -eq 'Running') {
        Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 2
    }
    & $BinaryDest uninstall --config $ConfigPath 2>&1 | Out-Null
    Start-Sleep -Seconds 1
}

& $BinaryDest install --config $ConfigPath
if ($LASTEXITCODE -ne 0) {
    throw "Service install failed (exit code $LASTEXITCODE). Check $BinaryDest is a valid agent binary."
}
Write-OK "Service '$serviceName' installed."

# ── Step 7: Start service ─────────────────────────────────────────────────────
Write-Step '7/7' 'Starting service'
Start-Service -Name $serviceName
Start-Sleep -Seconds 3

$svc = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -eq 'Running') {
    Write-OK 'Service is running.'
} else {
    Write-Warn 'Service may not have started. Check:'
    Write-Warn "  Get-EventLog -LogName Application -Source '$serviceName' -Newest 10"
    Write-Warn "  or: $(Join-Path $InstallDir 'logs')"
}

# ── Summary ───────────────────────────────────────────────────────────────────
Write-Host ''
Write-Host '============================================================' -ForegroundColor Cyan
Write-Host '  Distributed Encoder Agent Installed!' -ForegroundColor Cyan
Write-Host '============================================================' -ForegroundColor Cyan
Write-Host ''
Write-Host "  Agent hostname  : $AgentHostname"
Write-Host "  Controller      : $ControllerAddress"
Write-Host "  Install dir     : $InstallDir"
Write-Host "  Config file     : $ConfigPath"
Write-Host "  Service name    : $serviceName"
Write-Host ''
Write-Host '  Next step: approve this agent.' -ForegroundColor Yellow
Write-Host '  Option A — web UI: open the web UI → Farm Servers → Approve.' -ForegroundColor Yellow
Write-Host '  Option B — CLI on the controller host:' -ForegroundColor Yellow
Write-Host "    docker compose exec controller /app/controller agent approve $AgentHostname" -ForegroundColor Yellow
Write-Host ''
Write-Host '  Useful commands:'
Write-Host "    Get-Service $serviceName"
Write-Host "    Stop-Service $serviceName"
Write-Host "    Start-Service $serviceName"
Write-Host "    Get-EventLog -LogName Application -Source '$serviceName' -Newest 20"
Write-Host '============================================================' -ForegroundColor Cyan
