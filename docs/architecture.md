# 架构与跨平台实现

## 🌍 跨平台支持

| 系统 | IP 转发 | NAT | 服务管理 | 本机 DNS 一键切 |
|---|---|---|---|---|
| **macOS** | `sysctl net.inet.ip.forwarding` | `pfctl` | `launchd` plist | ✅ `networksetup`（自动遍历所有活跃网卡） |
| **Linux** | `/proc/sys/net/ipv4/ip_forward` | `iptables MASQUERADE` | `systemd` unit | ⚠️ 手动（见 [device-setup.md](device-setup.md#linux-手动)） |
| **Windows** | 注册表 `IPEnableRouter=1` | ❌ 家用版没 RRAS/无 NAT | `schtasks` 计划任务 | ⚠️ 不推荐切（见 [device-setup.md](device-setup.md#windows)） |

编译产物 5 平台均通过：`darwin-arm64 / darwin-amd64 / linux-amd64 / linux-arm64 / windows-amd64`。

> Linux / Windows 欢迎 PR 贡献一键 DNS 切换 / 更完善的服务管理。

---

## 🧩 三层架构

```mermaid
flowchart TB
    subgraph L1["【主】gateway.*"]
        G1[LAN 网关<br/>IP 转发]
        G2[TUN<br/>劫持全部流量]
        G3[DNS 代理<br/>端口 53]
    end

    subgraph L2["【副】traffic.*"]
        T1[规则模式<br/>国内直连国外代理]
        T2[全局 / 直连]
        T3[广告拦截]
        T4[自定义规则]
    end

    subgraph L3["【拓展】source.*"]
        P1[单点代理]
        P2[机场订阅]
        P3[本地文件]
        P4[全局扩展脚本<br/>链式代理预设]
    end

    subgraph L4["【健壮性】supervisor"]
        H1[30s 源健康检查]
        H2[挂了自动切 direct]
        H3[恢复切回原 mode]
    end

    L1 --> L2 --> L3
    L2 -.被 H1-3 保护.-> L4

    style L1 fill:#ffe0e0,stroke:#cc0000
    style L2 fill:#fff0d9,stroke:#cc6600
    style L3 fill:#e0f0ff,stroke:#0066cc
    style L4 fill:#f0e6ff,stroke:#8800cc
```

---

## 🛠️ 手动编译

```bash
git clone https://github.com/Tght1211/lan-proxy-gateway
cd lan-proxy-gateway

make build            # 当前平台 → ./gateway
make install          # 装到 /usr/local/bin/gateway（需 sudo）
make test             # 全部单元测试
make build-all        # 交叉编译 darwin / linux / windows
```

国内网络拉依赖慢：`GOPROXY=https://goproxy.cn,direct go build -o gateway .`

---

## 📁 目录结构

```
cmd/                cobra 入口（install / start / stop / status / service …）
internal/
  app/              统一门面（console + cobra 共用）+ supervisor（代理源自愈）
  gateway/          【主】LAN 网关 + 设备接入指引
  traffic/          【副】规则 + 内置 ruleset + 自定义合并
  source/           【拓展】代理源 inline + 连通性测试
  engine/           mihomo 进程 + 渲染 + REST API（SelectNode / GroupDelay / SetMode）
  script/           goja 脚本执行器
    presets/        内嵌预设（链式代理 · 住宅 IP 落地）
  config/           v3 schema（v1/v2 自动迁移）
  platform/         跨平台（darwin/linux/windows）
  console/          菜单式交互 + 日志易读视图 + 显示宽度对齐
  mihomo/           下载 mihomo 内核
embed/
  template.yaml     mihomo config 模板
  webui/            metacubexd dist（2 MB+，go:embed all:）
legacy/v1/          v1 源码留档
```
