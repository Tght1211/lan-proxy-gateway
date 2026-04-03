$ErrorActionPreference = "Stop"

$Repo = "Tght1211/lan-proxy-gateway"
$Binary = "gateway.exe"
$InstallDir = "$env:LOCALAPPDATA\Programs\gateway"
$GHMirror = if ($env:GITHUB_MIRROR) { $env:GITHUB_MIRROR } else { "" }

function Detect-Mirror {
    if ($GHMirror) {
        Write-Host "使用指定镜像: $GHMirror" -ForegroundColor Green
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

# Add to persistent user PATH
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    Write-Host "已将 $InstallDir 添加到用户 PATH" -ForegroundColor Green
}

# Also refresh the current session so gateway is usable immediately without reopening terminal
if ($env:Path -notlike "*$InstallDir*") {
    $env:Path = "$env:Path;$InstallDir"
    Write-Host "当前会话 PATH 已更新，无需重启终端即可使用 gateway 命令" -ForegroundColor Green
}

Write-Host ""
Write-Host "安装成功!" -ForegroundColor Green
Write-Host "安装位置: $Target" -ForegroundColor Green
Write-Host ""
Write-Host "快速开始 (以管理员身份运行 PowerShell/终端):" -ForegroundColor Green
Write-Host "  gateway install    # 运行安装向导（下载 mihomo、配置代理）"
Write-Host "  gateway config     # 打开配置中心"
Write-Host "  gateway start      # 启动网关 (需要管理员权限)"
Write-Host "  gateway status     # 查看状态和出口网络"
Write-Host ""
Write-Host "提示: gateway start / stop / restart 需要在管理员终端中运行" -ForegroundColor Yellow
