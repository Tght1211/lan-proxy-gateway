$ErrorActionPreference = "Stop"

$Repo = "Tght1211/lan-proxy-gateway"
$Binary = "gateway.exe"
$InstallDir = "$env:LOCALAPPDATA\Programs\gateway"
$GHMirror = if ($env:GITHUB_MIRROR) { $env:GITHUB_MIRROR } else { "" }

function Detect-Mirror {
    if ($GHMirror) {
        Write-Host "使��指定镜像: $GHMirror" -ForegroundColor Green
        return
    }
    try {
        $null = Invoke-WebRequest -Uri "https://api.github.com" -TimeoutSec 5 -UseBasicParsing
        $script:GHMirror = ""
        return
    } catch {}

    Write-Host "直连 GitHub 超时，尝试镜像加速..." -ForegroundColor Yellow
    $mirrors = @(
        "https://hub.gitmirror.com/",
        "https://mirror.ghproxy.com/",
        "https://github.moeyy.xyz/",
        "https://gh.ddlc.top/"
    )
    foreach ($m in $mirrors) {
        try {
            $null = Invoke-WebRequest -Uri "${m}https://api.github.com" -TimeoutSec 5 -UseBasicParsing
            $script:GHMirror = $m
            Write-Host "使用镜像: $m" -ForegroundColor Green
            return
        } catch {}
    }
    throw "无法连接 GitHub 或任何镜像站。请设置: `$env:GITHUB_MIRROR = 'https://你的镜像/'"
}

Detect-Mirror

Write-Host "正在获取最新版本..." -ForegroundColor Green

$ApiUrl = "${GHMirror}https://api.github.com/repos/$Repo/releases/latest"
$Release = Invoke-RestMethod $ApiUrl
$Tag = $Release.tag_name
Write-Host "最新版本: $Tag" -ForegroundColor Green

$Asset = "gateway-windows-amd64.exe"
$Url = "${GHMirror}https://github.com/$Repo/releases/download/$Tag/$Asset"

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$Target = Join-Path $InstallDir $Binary
Write-Host "下载 $Asset..." -ForegroundColor Green
Invoke-WebRequest -Uri $Url -OutFile $Target -UseBasicParsing

# add to user PATH if not already there
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    Write-Host "已将 $InstallDir 添加到用户 PATH (重启终端生效)" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "安装成功!" -ForegroundColor Green
Write-Host "安装位置: $Target" -ForegroundColor Green
Write-Host ""
Write-Host "快速开始:" -ForegroundColor Green
Write-Host "  gateway install    # 安装向导"
Write-Host "  gateway start      # 启动网关 (需要管理员权限)"
Write-Host "  gateway status     # 查看状态"
