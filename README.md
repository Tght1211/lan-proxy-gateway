# LAN Proxy Gateway v2

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/Tght1211/lan-proxy-gateway)](LICENSE)

**把电脑变成局域网代理网关** —— 让不方便装代理 App 的设备（Switch / PS5 / Apple TV / 智能电视 / 手机）改个 "网关 + DNS" 就能科学上网。

面向**非编程玩家**：一键安装、3 步配置向导、全程中文菜单。

---

## 一图看懂

```
 Switch / PS5 / Apple TV / 智能电视 / 手机
                  │
                  │  把网关和 DNS 都改成这台电脑的 IP
                  ▼
          🖥  运行 gateway 的电脑
                  │
                  │  mihomo（规则 / 广告拦截 / TUN / DNS）
                  ▼
          ┌───────────┬───────────┬─────────┐
          ▼           ▼           ▼         ▼
       本机代理   订阅节点   Clash 文件   远程代理
       (7890)    (机场)      (.yaml)     (socks5)
```

电脑只要能科学上网（比如已经在跑 Clash Verge / Shadowrocket），把它变成网关就能让整屋设备一起享受。

---

## 三大能力

| 层级 | 能力 | 配置键 |
|---|---|---|
| **主功能** | LAN 网关（IP 转发 + TUN + DNS） | `gateway.*` |
| **副功能** | 流量控制（规则 / 全局 / 直连 + 广告拦截） | `traffic.*` |
| **拓展** | 代理端口（本机已有 / 订阅 / 文件 / 远程 / 无） | `source.*` |

---

## 安装（3 条命令）

### 编译

```bash
git clone https://github.com/Tght1211/lan-proxy-gateway
cd lan-proxy-gateway
make install                 # Mac/Linux：需要 sudo
```

Windows：

```powershell
go build -ldflags "-s -w" -o gateway.exe .
# 把 gateway.exe 放到 PATH 下
```

### 首次运行

```bash
sudo gateway install         # 下载 mihomo + 打开 3 步向导
```

向导会依次问你：

```
步骤 1 / 3  局域网网关
  启用 LAN 共享网关？(Y/n)
  启用 TUN？           (Y/n)
  启用 DNS 代理？      (Y/n)

步骤 2 / 3  流量控制模式
  1) 规则模式 (推荐)    国内直连 + 国外代理
  2) 全局模式
  3) 直连模式
  开启广告拦截？ (Y/n)

步骤 3 / 3  代理端口来源
  1) 本机已有代理端口   ← 已经在跑 Clash Verge 的用户选这个
  2) 订阅链接
  3) Clash 配置文件
  4) 远程单点代理
  5) 暂不配置
```

向导结束会把配置写到 `~/.config/lan-proxy-gateway/gateway.yaml`（Windows: `%APPDATA%\lan-proxy-gateway\gateway.yaml`），并打印出设备接入指引。

### 启动

```bash
sudo gateway                 # 进入主菜单
sudo gateway start           # 后台启动（供开机自启用）
sudo gateway stop
sudo gateway status
```

### 让其他设备接入

把设备的：

- `网关 (Gateway)` 改成 **运行 gateway 的电脑的局域网 IP**
- `DNS 服务器` 改成 **同一个 IP**

保存后重连网络即可。

> **⚠ 前提一：本机 TUN 必须开启**
>
> 光改网关让流量"流经电脑"还不够 —— 电脑默认只做**普通路由转发**，
> Switch/PS5 照样被墙。**TUN 才是劫持并让流量走代理的关键**。
>
> 因此本项目 TUN 默认开启，不建议关闭。
> 只有当你**仅给手机/电脑用、并愿意手动填代理服务器 = 本机IP:7890**
> 时才能关 TUN —— 这种场景下 Switch/PS5 是接入不了代理的。
>
> **⚠ 前提二：DNS 代理的处理**
>
> | 场景 | 设备的 DNS 设置 |
> |---|---|
> | 本机 DNS 代理【开】（默认） | 指向本机 IP（省事） |
> | 本机 DNS 代理【关】（比如 Clash Verge 已占 :53） | 可继续指向本机 IP（由占用方回答），或改成 `114.114.114.114` |
> | 本机 DNS 代理关 **且** 本机 :53 没人接管 | 设备必须单独设一个能用的 DNS，否则**完全断网** |
>
> TUN 模式下关 DNS 还会让 fake-ip 机制失效，劫持可能不完整。如不确定就保持默认开着。

---

## 完整命令

所有操作都可以在**主菜单**里完成，不用记命令。下面是无人值守场景用的 cobra 命令：

| 命令 | 作用 |
|---|---|
| `gateway` | 进入主菜单（或 3 步向导） |
| `gateway install` | 下载 mihomo + 首次向导 |
| `gateway start` | 非交互启动（供系统服务调用） |
| `gateway stop` | 停止 |
| `gateway status` | 一次性输出当前状态 |
| `gateway service install` | 安装为开机自启（launchd / systemd / schtasks） |
| `gateway service uninstall` | 卸载系统服务 |
| `gateway service status` | 查看服务状态 |

---

## 跨平台支持

- **macOS** (主要测试平台): `sysctl` 开启 IP 转发，launchd 服务。
- **Linux**: `/proc/sys/net/ipv4/ip_forward` + iptables MASQUERADE，systemd 单元。
- **Windows**: 注册表 `IPEnableRouter` + 计划任务开机自启。`mihomo` 的 TUN 模式会创建 `Mihomo` 虚拟网卡接管路由。

三平台代码都能编译通过。目前**仅在 macOS 做过完整功能测试**；Linux / Windows 通过了编译和单元测试，但建议先在小规模局域网验证再推广，遇到问题请开 issue。

---

## 配置文件

完整示例见 [`gateway.example.yaml`](gateway.example.yaml)。最小示例：

```yaml
version: 2
gateway:
  enabled: true
  tun: { enabled: true, bypass_local: false }
  dns: { enabled: true, port: 53 }
traffic:
  mode: rule
  adblock: true
  rulesets:
    china_direct: true
    apple: true
    nintendo: true
    global: true
    lan_direct: true
source:
  type: external
  external:
    server: 127.0.0.1
    port: 7890
    kind: http
runtime:
  ports: { mixed: 7890, redir: 7892, api: 9090 }
```

v1 配置会在首次加载时自动升级到 v2 并打印迁移报告。

---

## 开发

```bash
make build        # 编译当前平台
make test         # 跑全部单元测试
make test-core    # 仅跑核心包测试（更快）
make build-all    # darwin/linux/windows 交叉编译
```

目录结构（v2）：

```
cmd/              cobra 入口（5 个命令）
internal/
  app/            统一门面：console + cobra 都调这里
  gateway/        【主】LAN 网关
  traffic/        【副】流量控制 + 内置规则集
  source/         【拓展】代理源（external/subscription/file/remote/none）
  engine/         mihomo 进程 + 渲染 + REST API
  config/         v2 schema + v1 迁移
  platform/       跨平台（darwin/linux/windows）
  console/        菜单式交互
  mihomo/         下载 mihomo 内核
embed/            mihomo yaml 模板
legacy/v1/        v1 源码留档（不参与编译）
```

---

## License

[MIT](LICENSE)
