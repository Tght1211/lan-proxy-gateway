$ErrorActionPreference = "Stop"

$Repo = "Tght1211/lan-proxy-gateway"
$Binary = "gateway.exe"
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\gateway"
$GHMirror = if ($env:GITHUB_MIRROR) { $env:GITHUB_MIRROR.Trim() } else { "" }
$RequestHeaders = @{ "User-Agent" = "lan-proxy-gateway-install" }
$ProbeTimeoutSec = 10

function Write-Info {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Yellow
}

function Path-Contains {
    param(
        [string]$PathValue,
        [string]$Entry
    )

    if (-not $PathValue) {
        return $false
    }

    foreach ($item in ($PathValue -split ";")) {
        if ($item.Trim() -ieq $Entry) {
            return $true
        }
    }

    return $false
}

function Add-CurrentSessionPath {
    if (-not (Path-Contains -PathValue $env:Path -Entry $InstallDir)) {
        $env:Path = "$InstallDir;$env:Path"
    }
}

function Download-WithFallback {
    param(
        [string[]]$Urls,
        [string]$OutFile
    )

    $lastErrorMessage = ""
    foreach ($candidate in $Urls) {
        try {
            Invoke-WebRequest -Uri $candidate -Headers $RequestHeaders -OutFile $OutFile -UseBasicParsing
            return
        } catch {
            $lastErrorMessage = $_.Exception.Message
            if (Test-Path $OutFile) {
                Remove-Item -LiteralPath $OutFile -Force -ErrorAction SilentlyContinue
            }
        }
    }

    throw "Failed to download release asset. Last error: $lastErrorMessage"
}

function Detect-Mirror {
    if ($GHMirror) {
        Write-Info "Using custom mirror: $GHMirror"
        return
    }

    try {
        $null = Invoke-WebRequest -Uri "https://api.github.com" -Headers $RequestHeaders -TimeoutSec $ProbeTimeoutSec -UseBasicParsing
        $script:GHMirror = ""
        return
    } catch {}

    Write-Warn "GitHub direct access timed out. Trying mirrors..."
    $mirrors = @(
        "https://hub.gitmirror.com/",
        "https://mirror.ghproxy.com/",
        "https://github.moeyy.xyz/",
        "https://gh.ddlc.top/"
    )

    foreach ($m in $mirrors) {
        try {
            $null = Invoke-WebRequest -Uri "${m}https://api.github.com" -Headers $RequestHeaders -TimeoutSec $ProbeTimeoutSec -UseBasicParsing
            $script:GHMirror = $m
            Write-Info "Using mirror: $m"
            return
        } catch {}
    }

    throw "Could not reach GitHub or any known mirror. Set `$env:GITHUB_MIRROR to your preferred mirror and retry."
}

Detect-Mirror

Write-Info "Fetching latest release..."
$ApiUrl = "${GHMirror}https://api.github.com/repos/$Repo/releases/latest"
$Release = Invoke-RestMethod -Uri $ApiUrl -Headers $RequestHeaders
$Tag = $Release.tag_name
Write-Info "Latest release: $Tag"

# v3.0.0 起资产名：gateway-<tag>-windows-amd64.zip（压缩包里含 gateway-<tag>-windows-amd64.exe）
$Asset = "gateway-$Tag-windows-amd64.zip"
$Url = "${GHMirror}https://github.com/$Repo/releases/download/$Tag/$Asset"
$AssetUrls = @($Url)
if (-not $GHMirror) {
    $AssetUrls += @(
        "https://hub.gitmirror.com/https://github.com/$Repo/releases/download/$Tag/$Asset",
        "https://mirror.ghproxy.com/https://github.com/$Repo/releases/download/$Tag/$Asset",
        "https://github.moeyy.xyz/https://github.com/$Repo/releases/download/$Tag/$Asset",
        "https://gh.ddlc.top/https://github.com/$Repo/releases/download/$Tag/$Asset"
    )
}

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$TmpDir = Join-Path $env:TEMP ("gateway-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null
$ZipPath = Join-Path $TmpDir $Asset

try {
    Write-Info "Downloading $Asset..."
    Download-WithFallback -Urls $AssetUrls -OutFile $ZipPath

    Write-Info "Extracting..."
    Expand-Archive -Path $ZipPath -DestinationPath $TmpDir -Force
    $ExtractedExe = Join-Path $TmpDir "gateway-$Tag-windows-amd64.exe"
    if (-not (Test-Path $ExtractedExe)) {
        throw "Extracted binary not found: $ExtractedExe"
    }

    $Target = Join-Path $InstallDir $Binary
    Move-Item -Path $ExtractedExe -Destination $Target -Force
} finally {
    if (Test-Path $TmpDir) {
        Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
    }
}

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (-not $UserPath) {
    $UserPath = ""
}

if (-not (Path-Contains -PathValue $UserPath -Entry $InstallDir)) {
    $NewUserPath = if ($UserPath) { "$UserPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", $NewUserPath, "User")
    Write-Warn "Added $InstallDir to user PATH for future terminals."
}

Add-CurrentSessionPath

Write-Host ""
Write-Info "Install complete."
Write-Info "Installed to: $Target"
Write-Host ""
Write-Info "下一步（管理员 PowerShell）:"
Write-Host "  gateway install    # 下载 mihomo 内核 + 配置向导 + 启动"
Write-Host "  gateway            # 以后进主菜单"
Write-Host "  gateway status     # 非交互查看状态"
Write-Host ""
Write-Info "开机自启:"
Write-Host "  gateway service install   # 注册为计划任务"
