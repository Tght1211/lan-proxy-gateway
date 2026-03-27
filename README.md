# LAN Proxy Gateway

**不刷固件、不买软路由，一条命令把你的电脑变成全屋科学上网网关。**

支持 **macOS / Linux / Windows**。目标是让小白也能一键部署：安装 -> 初始化 -> 启动 -> 设备改网关。

## 一键安装（推荐，先走这条）

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | bash
```

国内网络不稳定可加镜像：

```bash
GITHUB_MIRROR=https://mirror.ghproxy.com/ bash -c "$(curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh)"
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.ps1 | iex
```

## 安装后 3 步验证（必须执行）

```bash
gateway version
gateway install
gateway status
```

预期：
- `gateway version` 能输出版本号（不是 unknown command）
- `gateway install` 能进入安装向导
- `gateway status` 能输出网关状态

## 初始化与启动

```bash
# 交互式初始化（会自动检查/下载 mihomo）
gateway install

# 启动网关
sudo gateway start

# 查看状态
gateway status
```

## 高级命令（新版本）

```bash
sudo gateway update
gateway switch best --region HK,JP --dry-run
gateway ui
```

## 配置文件（推荐使用 gateway.yaml）

安装向导会生成 `gateway.yaml`。新用户请优先使用它；旧 `.secret` 仅兼容保留。

```yaml
proxy_source: url
subscription_url: "https://..."
subscription_name: subscription
ports:
  mixed: 7890
  redir: 7892
  api: 9090
  dns: 53
api_secret: ""
ui:
  listen: "127.0.0.1:9091"
regions:
  enabled: false
  include: ["HK", "JP"]
  auto_switch: true
  strategy: "latency"
```

> 旧 `.secret` 可通过 `gateway install` 或 `sudo gateway update` 自动迁移。

## 国内下载失败怎么办（mihomo / release）

1) 先换镜像重试（推荐）
```bash
GITHUB_MIRROR=https://mirror.ghproxy.com/ gateway install
```

2) 单独安装 mihomo（Linux 兜底）
```bash
bash install-mihomo.sh
```

3) 再执行初始化
```bash
gateway install
```

## 常见问题（重点）

### Q1: `unknown command "update"` / `unknown command "version"`
你命中的是旧二进制。执行：

```bash
which gateway
gateway version
```

如果版本旧，重新跑安装脚本，或手动覆盖 `/usr/local/bin/gateway`。

### Q2: `switch best --region` 报 unknown flag
同上，说明二进制版本太旧。先升级后再执行。

### Q3: 代码更新了但本机命令没变
请确认：
- 命中路径是否正确（`which gateway`）
- `gateway version` 是否等于 Release 最新 tag
- `sudo gateway update` 执行后是否有版本变化

## 备选安装方式

### 手动下载 Release 二进制
到 Releases 下载对应平台资产，放入 PATH 并赋予可执行权限。

### 源码编译

```bash
git clone https://github.com/Tght1211/lan-proxy-gateway.git
cd lan-proxy-gateway
make install
```

## 项目结构（核心）

```text
cmd/                    # CLI 命令
internal/config/        # 配置结构与迁移
internal/mihomo/        # mihomo API 与下载逻辑
embed/template.yaml     # mihomo 运行配置模板
install.sh / install.ps1# 一键安装脚本
```

## License

[MIT](LICENSE)
