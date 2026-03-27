$ErrorActionPreference = "Stop"

$Repo = "Tght1211/lan-proxy-gateway"
$Binary = "gateway.exe"
$InstallDir = "$env:LOCALAPPDATA\Programs\gateway"
$GHMirror = if ($env:GITHUB_MIRROR) { $env:GITHUB_MIRROR } else { "" }

function Write-Info($m) { Write-Host $m -ForegroundColor Green }
function Write-WarnMsg($m) { Write-Host $m -ForegroundColor Yellow }

function Detect-Mirror {
    if ($GHMirror) {
        Write-Info "使用指定镜像: $GHMirror"
        return
    }
    try {
        $null = Invoke-WebRequest -Uri "https://api.github.com" -TimeoutSec 6 -UseBasicParsing
        $script:GHMirror = ""
        return
    } catch {}

    Write-WarnMsg "直连 GitHub 超时，尝试镜像加速..."
    $mirrors = @(
        "https://hub.gitmirror.com/",
        "https://mirror.ghproxy.com/",
        "https://github.moeyy.xyz/",
        "https://gh.ddlc.top/"
    )
    foreach ($m in $mirrors) {
        try {
            $null = Invoke-WebRequest -Uri "${m}https://api.github.com" -TimeoutSec 6 -UseBasicParsing
            $script:GHMirror = $m
            Write-Info "使用镜像: $m"
            return
        } catch {}
    }
    throw "无法连接 GitHub 或镜像站。请设置: `$env:GITHUB_MIRROR='https://你的镜像/'"
}

Detect-Mirror

Write-Info "正在获取最新版本..."
$ApiUrl = "${GHMirror}https://api.github.com/repos/$Repo/releases/latest"
$Release = Invoke-RestMethod $ApiUrl
$Tag = $Release.tag_name
if (-not $Tag) { throw "无法获取最新版本号" }
Write-Info "最新版本: $Tag"

$Asset = "gateway-windows-amd64.exe"
$assets = @($Release.assets | ForEach-Object { $_.name })
if ($assets -notcontains $Asset) {
    throw "最新版本缺少当前平台资产: $Asset"
}

$Url = "${GHMirror}https://github.com/$Repo/releases/download/$Tag/$Asset"

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$Target = Join-Path $InstallDir $Binary
Write-Info "下载 $Asset..."
Invoke-WebRequest -Uri $Url -OutFile $Target -UseBasicParsing

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    Write-WarnMsg "已将 $InstallDir 添加到用户 PATH（重启终端生效）"
}

$VersionOutput = ""
try {
    $VersionOutput = & $Target version 2>$null
    if (-not $VersionOutput) { $VersionOutput = & $Target --version 2>$null }
} catch {}
if (-not $VersionOutput) {
    throw "安装后版本校验失败：无法执行 gateway version"
}

$Resolved = ""
try { $Resolved = (Get-Command gateway -ErrorAction SilentlyContinue).Source } catch {}

Write-Host ""
Write-Info "安装成功!"
Write-Info "版本: $VersionOutput"
if ($Resolved) { Write-Info "当前命中: $Resolved" }
Write-Host ""
Write-Info "安装后验证（必做）:"
Write-Host "  gateway version"
Write-Host "  gateway install"
Write-Host "  gateway status"
