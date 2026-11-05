## 下载哪个文件？

**Mac 第一次使用：下载下面第一行的 DMG 即可，M 系列和 Intel 都能用。** 软件安装在提供网络的电脑上；手机、电视和游戏机只需连接它的网络，不安装本项目。

| 你的设备 / 用途 | 下载文件（点击下载） | 安装方式 |
|---|---|---|
| **Mac 图形 App，macOS 13+（推荐）** | **[LANProxyGateway-{{TAG}}-macos-universal.dmg]({{BASE}}/LANProxyGateway-{{TAG}}-macos-universal.dmg)** | Apple Silicon / Intel 通用；打开 DMG，拖入“应用程序”，再从那里启动 |
| Mac M 系列，仅命令行 | [gateway-darwin-arm64.tar.gz]({{BASE}}/gateway-darwin-arm64.tar.gz) | 解压后安装核心；没有图形界面 |
| Mac Intel，仅命令行 | [gateway-darwin-amd64.tar.gz]({{BASE}}/gateway-darwin-amd64.tar.gz) | Intel 64 位使用 amd64 包；没有图形界面 |
| Linux，Intel / AMD 64 位 | [gateway-linux-amd64.tar.gz]({{BASE}}/gateway-linux-amd64.tar.gz) | 完整 Linux 命令行；`uname -m` 为 `x86_64` / `amd64` |
| Linux，ARM 64 位 | [gateway-linux-arm64.tar.gz]({{BASE}}/gateway-linux-arm64.tar.gz) | 完整 64 位 Linux；`uname -m` 为 `aarch64` / `arm64` |
| AI Agent 扩展（可选） | [lan-proxy-gateway-skill.zip]({{BASE}}/lan-proxy-gateway-skill.zip) | 给支持 Skill 的本机 Agent；不是 App，也不替代核心 |
| 核对下载完整性（可选） | [SHA256SUMS]({{BASE}}/SHA256SUMS) | SHA-256 校验清单，不是安装包 |
| 手机 / 平板 / PS5 / Switch / Apple TV / 电视 | **不需要安装包** | 连接主机创建的热点，或按设备能力配置网关 / HTTP 代理 |
| Windows / OpenWrt 作为网关主机 | 暂不支持 | 不要下载 Linux 包尝试在 Windows 直接运行 |

**附件中的其他文件：** `gateway-darwin-arm64`、`gateway-darwin-amd64`、`gateway-linux-arm64`、`gateway-linux-amd64` 是未压缩的命令行核心，与对应 `.tar.gz` 内的程序相同，二选一即可。GitHub 的 `Source code (zip / tar.gz)` 是源码，不是给普通用户安装的软件。`darwin` 代表 macOS，`amd64` 也适用于 Intel CPU。

**第一次用 Mac：** 安装 App → 配置已有 HTTP/SOCKS5 上游并测试 → “设备接入”选择一种方式 → 在设备上实际访问并查看新连接。有线 Mac 可用 Wi-Fi 热点；仅 Wi-Fi 上网时先考虑 HTTP/PAC。项目不提供节点或订阅。

**已安装用户：** 更新 App 后，若提示“更新并重启核心”，还需执行该步骤；会短暂断开设备连接。安装、接入和恢复网络说明请从 [项目首页]({{REPO_URL}}#readme) 进入。

---
