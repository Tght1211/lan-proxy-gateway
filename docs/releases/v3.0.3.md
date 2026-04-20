# LAN Proxy Gateway v3.0.3

这一版重梳理首次安装 → 可用的交互流程：**一条 `curl | bash` 走到能用**，中间不再让用户记命令。

## 重点变更

### 1. `install.sh` 装完自动接入配置向导

以前：装完只打印「下一步敲 `sudo gateway install`」，用户要自己记。
现在：装完二进制后，只要当前 shell 有终端（`/dev/tty` 可用，`curl | bash` 也算），脚本会 `exec sudo gateway install` 直接进入向导。非交互场景（CI / 无终端）仍然只打印提示。

### 2. `gateway install` 装完直接退出，不再进主菜单

以前：配完 + 启动 + 打印接入指引后，把用户塞进主菜单，大多数人按 Q 走人。
现在：打印接入指引后直接退出 shell，mihomo 作为独立进程在后台跑。靠 v3.0.1 引入的 reattach 机制，之后 `sudo gateway` 能无缝接回。

### 3. 向导末尾问「要开机自启吗」（默认 y）

以前：需要用户单独再跑 `sudo gateway service install`，很多人装完就忘了。
现在：向导最后一步直接问 y/n，默认 y，顺手装 launchd / systemd / schtasks 任务。

### 4. `gateway start` 默认后台化，`--foreground` 留给 service

以前：`sudo gateway start` 会前台阻塞等 `Ctrl+C`（给 launchd 用的），普通用户以为「卡住了」。
现在：默认启动后立即返回 shell；launchd plist / systemd unit / schtasks 命令统一改成 `start --foreground` 保留阻塞语义。

## 新的 happy path

```
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | bash
  ↓ 自动进入向导（会请求一次 sudo 密码）
  ↓ 问代理源（只需要选一项）
  ↓ 装 mihomo 内核 + GeoIP / GeoSite
  ↓ 启动 mihomo
  ↓ 问「要开机自启吗」(Y/n)
  ↓ 打印接入指引（本机 IP 当网关 + DNS）
  → 回到 shell，mihomo 已经在后台跑了
```

之后想调整：`sudo gateway` 进主菜单。想停：`sudo gateway stop`。

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.3/gateway-darwin-arm64) / [gateway-darwin-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.3/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.3/gateway-darwin-amd64) / [gateway-darwin-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.3/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.3/gateway-linux-amd64) / [gateway-linux-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.3/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.3/gateway-linux-arm64) / [gateway-linux-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.3/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.3/gateway-windows-amd64.exe) / [gateway-windows-amd64.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.3/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.3/SHA256SUMS)

## 兼容性 / 升级提示

- 已经装过 service 的用户：原 plist/unit 里是 `start`（前台），升级到 v3.0.3 后 `start` 默认后台了，老 service 会变成「launchd 以为进程早退」→ 被 KeepAlive 反复重启。**升级后建议重装 service**：
  ```bash
  sudo gateway service uninstall
  sudo gateway service install
  ```
- 脚本里用 `gateway start` 的：如需保持前台阻塞行为，改成 `gateway start --foreground`。
