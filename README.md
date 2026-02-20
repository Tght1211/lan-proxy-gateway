# LAN Proxy Gateway

> 把你的 Mac 变成局域网透明代理网关，让 Switch、智能电视等不能装代理的设备轻松科学上网。

```
                        ┌─────────────┐
                        │   互联网     │
                        └──────┬──────┘
                               │
                        ┌──────┴──────┐
                        │   路由器     │
                        │ 192.168.x.1 │
                        └──────┬──────┘
                               │ 局域网
              ┌────────────────┼────────────────┐
              │                │                │
     ┌────────┴────────┐      │       ┌────────┴────────┐
     │   Mac (网关)     │      │       │   其他设备       │
     │  mihomo TUN     │      │       │  自动获取 IP     │
     │  192.168.x.2    │      │       └─────────────────┘
     └─────────────────┘      │
                              │
              ┌───────────────┼───────────────┐
              │               │               │
     ┌────────┴──────┐ ┌─────┴───────┐ ┌─────┴───────┐
     │  Switch       │ │  手机/平板   │ │  智能电视    │
     │  网关→Mac IP  │ │  网关→Mac IP │ │  网关→Mac IP│
     └───────────────┘ └─────────────┘ └─────────────┘
```

## 特性

- **一键安装** - 交互式向导，输入订阅链接就能用
- **隐私安全** - 订阅链接等敏感信息不会进入 Git
- **自动检测** - 自动识别网卡、IP、CPU 架构
- **TUN 模式** - 透明代理，设备无需安装任何软件
- **分流规则** - Nintendo 走代理，国内走直连，广告拦截

## 快速开始

### 前置要求

- macOS (Apple Silicon 或 Intel)
- [Homebrew](https://brew.sh/)

### 安装

```bash
git clone https://github.com/yourname/lan-proxy-gateway.git
cd lan-proxy-gateway
bash install.sh
```

安装向导会：
1. 自动安装 mihomo（如果没有）
2. 下载 GeoIP/GeoSite 数据
3. 引导你输入订阅链接
4. 检测网络环境并生成配置

### 使用

```bash
# 启动网关
sudo ./start.sh

# 查看状态
./status.sh

# 停止网关
sudo ./stop.sh
```

### 设备配置

启动后，将其他设备的网络设置改为**手动**：

| 设置项 | 值 |
|--------|-----|
| IP 地址 | 同网段任意可用 IP（如 192.168.x.100） |
| 子网掩码 | 255.255.255.0 |
| 网关 | Mac 的局域网 IP（start.sh 会显示） |
| DNS | Mac 的局域网 IP（同上） |

> Switch 详细设置请看 [Switch 设置指南](docs/switch-setup.md)

## 项目结构

```
lan-proxy-gateway/
├── install.sh          # 一键安装向导
├── start.sh            # 启动网关
├── stop.sh             # 停止网关
├── status.sh           # 状态面板
├── lib/
│   ├── common.sh       # 通用工具函数
│   ├── detect.sh       # 自动检测
│   └── config.sh       # 配置渲染
├── config/
│   └── template.yaml   # mihomo 配置模板
├── .secret.example     # 敏感配置示例
└── docs/
    └── switch-setup.md # Switch 设置指南
```

### 隐私保护

| 文件 | Git 追踪 | 说明 |
|------|---------|------|
| `config/template.yaml` | Yes | 配置模板，`{{变量}}` 占位 |
| `.secret` | **No** | 订阅 URL 等敏感信息 |
| `data/config.yaml` | **No** | 运行时生成的完整配置 |

你的订阅链接只存在于本地 `.secret` 文件中，永远不会被提交到 Git。

## 工作原理

1. mihomo 运行在 TUN 模式，创建虚拟网卡接管所有流量
2. Mac 开启 IP 转发，充当局域网网关
3. 其他设备将网关和 DNS 指向 Mac
4. 所有流量经过 mihomo 分流：国内直连、国外走代理
5. 代理节点通过 `proxy-providers` 从订阅链接远程拉取，不硬编码在配置中

## FAQ

### Q: 需要一直开着 Mac 吗？
A: 是的，Mac 充当网关角色，需要保持运行。关闭 Mac 或停止 mihomo 后，其他设备需要改回自动获取网络。

### Q: 支持哪些订阅格式？
A: 支持标准的 Clash/mihomo 订阅链��（大部分机场都提供）。

### Q: 为什么需要 sudo？
A: TUN 模式需要创建虚拟网卡和修改路由表，这些操作需要 root 权限。IP 转发也需要 root 权限。

### Q: 怎么切换节点？
A: 访问 `http://Mac的IP:9090/ui` 打开 mihomo Web 面板，或使用 `status.sh` 查看当前节点。

### Q: Switch 连上了但是不能上网？
A: 检查以下几点：
1. Mac 和 Switch 在同一局域网
2. `sudo ./start.sh` 是否执行成功
3. Switch 的网关和 DNS 是否都设为 Mac 的 IP
4. 运行 `./status.sh` 确认 mihomo 正在运行

## License

[MIT](LICENSE)
