# 下载与安装：我该选哪个文件？

[项目首页](../README.md) · [完整使用手册](user-guide.md)

**先看运行网关的电脑，不要按手机或游戏机的型号选安装包。** Mac 新手直接下载 universal DMG。接入网络的设备不安装本项目。

打开 [最新 Release](https://github.com/Tght1211/lan-proxy-gateway/releases/latest)，展开底部 **Assets**。以下文件名中“版本号”随 Release 改变；核对附件的基准版本为 v4.5.6。

## 安装包对照表

| 设备 / 用途 | 文件名 | 是什么 / 如何安装 |
|---|---|---|
| **Mac 图形界面（新手推荐）** | **`LANProxyGateway-v版本号-macos-universal.dmg`** | macOS 13+，Apple Silicon（M 系列）和 Intel 通用；双击后拖入“应用程序” |
| Mac M 系列，仅命令行 | `gateway-darwin-arm64.tar.gz` | 解压得到核心；没有 App 界面 |
| Mac Intel，仅命令行 | `gateway-darwin-amd64.tar.gz` | `amd64` 也包含 Intel 64 位 CPU；没有 App 界面 |
| Linux Intel / AMD 64 位 | `gateway-linux-amd64.tar.gz` | 适用于完整 Linux 主机；按下面的步骤安装 |
| Linux ARM 64 位 | `gateway-linux-arm64.tar.gz` | 系统须为 ARM64；例如运行完整 64 位 Linux 的 ARM 小主机 |
| 自动部署 / 已会用终端 | 上述 `gateway-*` 的无后缀文件 | 与对应 `.tar.gz` 内的可执行文件相同，二选一即可；不是图形安装器 |
| 已有网关，想让 AI Agent 管理 | `lan-proxy-gateway-skill.zip` | 可选扩展，不是网关程序，不替代 DMG / CLI |
| 校验下载文件 | `SHA256SUMS` | 文件校验清单，不是安装包 |
| 开发 / 编译源码 | GitHub 自动附带的 `Source code (zip / tar.gz)` | 源码归档，不是已编译软件，普通用户不用下载 |
| iPhone / Android / PS5 / Switch / Apple TV / 电视 | **不用下载** | 这些是接入设备，按 [接入手册](user-guide.md#wifi) 连接 |
| Windows / OpenWrt 作为主机 | 暂无支持的安装包 | 当前不是部署目标；Windows 作为支持 HTTP/PAC 的客户端是另一回事 |

`darwin` 就是 macOS。`universal` 表示一个 Mac App 同时支持两种 CPU；无需在 arm64 和 amd64 之间做选择。如果自行构建得到 `macos-arm64.dmg` 或 `macos-x86_64.dmg`，那是指定架构的构建，不应当作通用包分发。

## 不知道自己的 CPU？

- **Mac：** 苹果菜单 → 关于本机；“芯片 Apple M…”为 Apple Silicon，“处理器 Intel…”为 Intel。不过使用 DMG 时无需区分。
- **Linux：** 在终端执行 `uname -m`。`x86_64` / `amd64` 选 `linux-amd64`；`aarch64` / `arm64` 选 `linux-arm64`。其他输出（如 `armv7l`、`i686`）没有对应发布包，不要硬装。

## macOS 图形安装

1. 下载 universal DMG。
2. 双击打开，把 **LAN Proxy Gateway** 拖入 **Applications / 应用程序**。
3. 从“应用程序”启动，按需完成核心启动的管理员授权。
4. 继续 [配置已有代理](user-guide.md#upstream)，再接入一台设备。

DMG 已带核心。只有需要在终端操作时，才到 **设置 → CLI 与 Agent → 安装 CLI**；它会安装到 `/usr/local/bin/gateway`。打开受阻见 [常见问题](faq.md#open-app)。

<a id="linux"></a>
## Linux / 命令行用户：独立安装路线

这条路线不需要 Mac App。需要具备管理员权限的完整 Linux（使用 systemd 时可安装开机服务），或支持的 macOS；当前不以 Docker、OpenWrt 或普通家用路由器为部署目标。Linux 网络转发使用 iptables，需要系统具备对应能力。

### 1. 下载并安装

可以用项目安装脚本自动识别系统与架构。在主机终端依次执行：

```bash
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh -o /tmp/lan-proxy-gateway-install.sh
bash /tmp/lan-proxy-gateway-install.sh
gateway --version
```

脚本会下载已发布的核心。若提示 `command not found`，查看安装脚本输出的安装位置及 PATH 提示，或重新打开终端。

也可以手动安装。下面以 **Linux amd64** 为例；先把同一 Release 的 `gateway-linux-amd64.tar.gz` 和 `SHA256SUMS` 下载到同一个目录，在该目录运行：

```bash
# 只校验本次下载的文件；应显示 OK
awk '$2 == "gateway-linux-amd64.tar.gz" {print}' SHA256SUMS | sha256sum -c -
tar -xzf gateway-linux-amd64.tar.gz
sudo install -m 755 gateway-linux-amd64 /usr/local/bin/gateway
gateway --version
```

ARM64 请把上面三个文件名中的 `amd64` 换成 `arm64`。Mac CLI 使用对应的 `darwin-*` 文件，校验命令用 `shasum -a 256 -c -`。校验失败或没有匹配记录时，重新下载正确文件，不要继续安装。

### 2. 初始化、启动

```bash
sudo gateway install
```

按向导填写已经可用的 HTTP/SOCKS5 上游地址，选择是否安装开机服务。此操作会配置网络转发，需要管理员权限。代理软件仍需另外运行，本项目不导入订阅或替你选节点。

已有配置时会复用，不用删配置来重新开始。若只是调整设备出口，可执行（以下端口仅作示例）：

```bash
gateway egress proxy --type http --host 127.0.0.1 --port 7897
gateway status
```

`egress` 配置设备流量的出口；不要为了接入设备额外运行会修改 Mac 系统代理的 `system-proxy` 命令。

### 3. 接入设备并验证

CLI 主线使用 [静态网关接入](user-guide.md#static)：确认主机运行状态、DNS 配置、局域网 IP 和真实子网后设置设备，访问网页再检查状态或记录。`gateway status` 给出的候选 IP 只是建议，不保证未占用；掩码也应服从真实网络。

macOS 的互联网共享热点是另一种接入方式，Linux 安装本 CLI 不会自动获得 macOS 的 Wi-Fi 热点向导。

### 4. 日常命令

```bash
gateway status                  # 查看状态与配置位置
sudo gateway restart            # 重启，当前连接会断开
sudo gateway stop               # 停止转发
gateway service status          # 查看开机服务状态
gateway update                  # 检查并更新核心
```

更新后按命令提示确认服务已重启，再验证设备新连接。更多见 [命令说明](commands.md)。停止后要让静态网关设备恢复自动 IP / DNS 或原来的网络设置。

## 核对 DMG 的下载完整性（可选）

下载同一版本的 DMG 和 `SHA256SUMS`。在下载目录执行：

```bash
shasum -a 256 LANProxyGateway-v4.5.6-macos-universal.dmg
```

将输出与 `SHA256SUMS` 中**同名文件**的一整行哈希对照。版本号随下载版本替换。校验和用于核对文件完整性，不等于 Apple 公证。

## 更新与备份

App 用户按 [更新 App 和核心](user-guide.md#update) 操作。CLI 用户保留配置，再执行 `gateway update`；从 v3 迁移请先看 [迁移说明](migration-v4.md)。不要用删除整个配置目录的方式处理普通版本升级。
