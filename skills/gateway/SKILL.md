# LAN Proxy Gateway · AI Skill

面向 AI agent（Cursor、Claude Code、Copilot、Codex 等）的全功能操作手册。
人类用户也可作为速查手册使用。

## 文件结构

```
skills/
├── SKILL.md              ← 本文件：入口 + 命令速查
├── api-reference.md      ← /api/stats 完整响应结构与字段说明
├── scripts/              ← 开箱即用的监控与诊断脚本
│   ├── overview.sh       ← 网络概览
│   ├── learn-watch.sh    ← 自动学习观察
│   ├── failed-conns.sh   ← 连接失败诊断
│   ├── device-traffic.sh ← 设备流量排行
│   └── device-protect.sh ← 设备断路器状态
├── troubleshooting.md    ← 故障诊断速查表
└── config-reference.md   ← 配置文件完整字段参考
```

按需阅读对应文件，不需要一次性加载全部。

---

## 快速开始

```bash
# 确认网关运行中
gateway status --json

# 拉全量快照（所有监控数据的唯一入口）
curl -s http://127.0.0.1:19090/api/stats | python3 -m json.tool
```

> 端口默认 `19090`，实际值以 `gateway status --json` 的 `ports.api` 为准。

---

## 命令速查

### 生命周期

```bash
sudo gateway install                  # 初始化 + 启动 + 可选系统服务
sudo gateway start                    # 后台守护进程
sudo gateway start --foreground       # 前台调试
sudo gateway restart
sudo gateway stop
gateway status [--json]
```

### 开机自启

```bash
sudo gateway service install
sudo gateway service uninstall
gateway service status
```

### 出口配置

```bash
gateway system-proxy on --type socks5 --host 127.0.0.1 --port 7897
gateway system-proxy on --type http   --host 127.0.0.1 --port 7897
gateway system-proxy off
gateway system-proxy status [--json]
```

### 分流规则

```bash
gateway routing list [--json]
sudo gateway routing set --rules-json '<json array>'
```

规则类型：`src-ip`、`domain`、`domain-suffix`、`ip-cidr`
动作：`proxy`、`direct`、`reject`

`src-ip` 是设备级前置策略，优先于所有域名规则。
`reject` 对 src-ip 会同步到防火墙阻断该设备全部转发。

#### 追加一条规则（不覆盖已有）

```bash
EXISTING=$(gateway routing list --json)
echo "$EXISTING" | python3 -c "
import sys, json
rules = json.load(sys.stdin)
rules.append({'type':'domain-suffix','value':'example.com','action':'direct'})
print(json.dumps(rules))
" | xargs -0 sudo gateway routing set --rules-json
```

### 更新

```bash
gateway update                        # 保留代理环境
gateway update --yes                  # 自动确认
```

---

## API 端点

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/stats` | GET | 全量快照：连接、流量、DNS、健康、学习、设备自适应 |
| `/api/health` | GET | 轻量健康探针 |
| `/api/reload` | POST | 热重载配置 |

完整响应结构和字段说明见 → [api-reference.md](api-reference.md)

---

## 设备接入

设备手动设置静态 IP，网关和 DNS 都指向本机：

| 项目 | 填写 |
|------|------|
| IP 地址 | 同网段未占用地址（每台设备不同） |
| 子网掩码 | `255.255.255.0`（Android 前缀 24） |
| 网关 | 本机 IP |
| DNS 1 | 本机 IP |
| DNS 2 | 同上或留空 |

`gateway status` 查看本机 IP。各设备图文步骤见 [docs/device-setup.md](../docs/device-setup.md)。

---

## 架构速记

```
设备 → pf/iptables 捕获 TCP → relay 恢复原始目标 → direct/proxy dialer
设备 DNS → 内置 DNS(fake-IP) → 上游 DNS
UDP fake-IP → UDP relay → 直连真实目标（游戏语音/视频）
QUIC(UDP/443) → 拒绝 → 浏览器回退 TCP → 可代理
```

- macOS 用 pf + `DIOCNATLOOK`，Linux 用 iptables + `SO_ORIGINAL_DST`
- 自动学习：代理失败 → 直连成功 → 24h 内 3 次 → 生成直连规则（按服务自动分组）
- 设备断路器：2 分钟内 5 个不同目标代理失败 → 该设备临时直连 15 分钟
- 连接记录保留 10 分钟，采样点 5 分钟，全部纯内存，重启清零
- API 仅监听 `127.0.0.1`，只能本机访问

---

## 注意事项

- 启动 / 停止 / 规则写入需要 `sudo`
- `POST /api/reload` 不需要 sudo
- 本项目不提供代理节点 / 订阅 / 规则集，那些由上游代理软件负责
- 日志：`~/.config/lan-proxy-gateway/gateway.log`
- 配置：`~/.config/lan-proxy-gateway/gateway.yaml`（完整字段见 [config-reference.md](config-reference.md)）
- 初次安装引导见 [docs/ai-setup.md](../docs/ai-setup.md)
- 故障排查见 [troubleshooting.md](troubleshooting.md)
