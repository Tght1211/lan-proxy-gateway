# Linux CLI / Desktop 对齐检查与 Docker 实测

检查日期：2026-11-06（Asia/Shanghai）。

结论：Linux CLI 具备大部分与 Desktop 共用的代理核心能力，但不能视为完整对齐并验证通过。已发布 v4.5.6 的常规 CLI 操作通过测试，真实容器接入暴露 fake-IP UDP 故障；受测 main 另外保留了已在发布版本修复的上游协议切换 panic。

## 范围与环境

- 受测主分支提交：`7dcd2c84d427fdd63373e14ac02f9893e21c4de5`。
- 已发布版本：GitHub Release v4.5.6 的 `gateway-linux-arm64`，实际下载并比对发行包 SHA256。
- SHA256：`12e77ebfb2da3ecf09a4adca246627109275e5b44e281bc8b498cc47412967f1`。
- 本机 macOS，Docker Desktop 提供 Linux ARM64 内核。测试镜像 `lan-proxy-gateway-linux-audit:local` 基于 Alpine 3.22，含 bash、Python、curl、iproute2、iptables 和 dig。
- 镜像内使用隔离的网络命名空间；主机未配置端口映射，项目文件只读挂载。网关测试容器使用 NET_ADMIN 和 `systempaths=unconfined`，以便在容器自己的命名空间修改转发状态。
- TCP/HTTP 上游为可控的本地模拟服务；不把其成功当作公网节点、真实设备或所有发行版的验证。
- 测试容器已清理。测试工具已整理至 `tools/linux-audit/`，本次证据随报告归档；尚未修复网关功能代码。

## 功能对照

| 能力 | Linux CLI 情况 |
| --- | --- |
| 启停、重启、状态、配置保存 | 有；v4.5.6 容器实测通过 |
| HTTP / SOCKS5 上游与直连出口 | 有；两种上游的真实请求链路和切换已测试；main 切换会 panic |
| HTTP 显式代理、CONNECT、认证、PAC | 有；v4.5.6 实测通过 |
| 域名 / IP / 设备策略 | 共用核心；CLI 以 JSON 管理整份规则，没有 Desktop 的交互编辑体验；实测域名代理和 IP 拒绝规则 |
| DNS / fake-IP / TCP 透明转发 | 有；独立客户端容器到网关容器的 DNS 53→1053、TCP REDIRECT、SO_ORIGINAL_DST 与上游转发通过 |
| fake-IP UDP | 有实现，但真实 Linux REDIRECT 路径失败，详见问题 1 |
| 自动学习 | 共用核心，CLI 有 accept/ignore/restore/undo/configure；实测 ignore/restore，其余有代码/单测支撑，未做完整端到端验收 |
| 连接、流量历史、设备、健康状态 | 核心 API / agent snapshot 可读；终端菜单没有 Desktop 的拓扑、图表、筛选等完整展示 |
| 系统服务 / 自更新 | 有 systemd 实现及 update 命令；本次未验证真实 systemd 开机自启、升级和回滚 |
| Mac Wi-Fi 互联网共享引导 | 不支持 Linux；没有等价的 Linux 热点管理流程 |
| 自动修改本机系统代理 / DNS | Linux 平台实现不支持；CLI 的 system-proxy 命令可退化为 LAN 出口管理，不能据此认为 Linux 系统代理被修改 |
| Agent Skill | 导出和辅助脚本测试通过 |

终端交互菜单目前只有四个主要入口：启停、设置代理、设备参数、最近日志。高级功能散落在命令、JSON 配置和 API 中，面向小白的操作体验未对齐 Desktop。

## 验证结果

1. main：macOS `go test -race ./...` 全部通过。
2. main：Linux amd64、arm64 主程序均交叉编译成功。amd64 未做运行验证。
3. main：Linux 测试编译检查失败，防火墙测试引用未定义的 `darwinManager`。`-exec /usr/bin/true` 仅用于编译检查，不代表执行测试。
4. main：排除不能编译的 firewall 包后，其他 11 个含测试的包已在 Linux ARM64 容器实际运行通过。最初跳过的 Python 辅助测试在补齐 Python 和 bash 后补测通过。
5. v4.5.6：`smoke.py` 的 28 个检查通过，覆盖初始化、状态、实际 iptables 应用、IP 转发开关、HTTP/CONNECT/PAC、认证、上游 HTTP/SOCKS5、规则热应用、fake-IP DNS、学习操作、Agent、重启、停止和清理，以及无 panic 检查。
6. main：同一流程前 27 项检查通过，但最后的“核心日志无 panic”检查失败。CLI 返回成功不能保证每次配置热应用都完成。
7. v4.5.6：第二个容器模拟客户端，DNS 接入与 TCP 透明转发通过，UDP 回包超时。
8. v4.5.6：从直连运行状态切到代理，API 的 `udp_relay` 仍为 null；重启后才出现监听器。

以上是本次范围内的结果，不是全功能、所有发行版或所有网络环境验收。

## 已确认问题

### 1. Linux fake-IP UDP 原始目标恢复错误（v4.5.6 与 main）

客户端发送到 `198.18.0.1:18083`。iptables 的 UDP REDIRECT 计数增加，API 中 UDP 会话数始终为 0，客户端接收超时。

为确认机制，在相同命名空间插入一条独立诊断 REDIRECT 规则并使用 Python `recvmsg` + `IP_RECVORIGDSTADDR` 读取控制消息：返回的是 `172.17.0.2:18883`（重定向后的本地 IP/诊断监听端口），而非原来的 `198.18.0.1:18083`。

当前 [udprelay_linux.go](../../internal/relay/udprelay_linux.go) 将其直接当作原始目标；[udprelay.go](../../internal/relay/udprelay.go) 检查目标不属于 fake-IP 网段后直接丢弃。该 Linux 平台文件在 main 与 v4.5.6 相同。

证据：[客户端测试](linux-audit-2026-11-06/transparent.txt)、[原始目标抓取结果](linux-audit-2026-11-06/origdst-result.json)、[服务日志](linux-audit-2026-11-06/transparent-server.txt)。

影响：需要通过 fake-IP 恢复真实目的地的 UDP 流量无法沿这条路径工作。不能泛化为所有 UDP 都失效；未拦截的普通 UDP 是另一条路径。

### 2. 直连切代理后未启动 UDP relay（v4.5.6 与 main，共用核心问题）

先 `start`（默认直连），再 `egress proxy`。防火墙已有 UDP 重定向到 17893 的规则，但 API `udp_relay` 为 null。`restart` 后才出现 `0.0.0.0:17893`。

[daemon_services.go](../../internal/app/daemon_services.go) 仅在启动时满足代理模式条件才创建 UDP relay；[daemon_config.go](../../internal/app/daemon_config.go) 热应用只更新已有服务，没有补建该监听器。相关逻辑在发布版本中也存在。

证据：[模式切换测试](linux-audit-2026-11-06/udp-transition.txt)。重启只绕过监听器缺失，不解决问题 1。

### 3. main 切换 HTTP → SOCKS5 出现 panic（v4.5.6 已修复）

运行中执行两种上游协议切换，日志出现：

```
sync/atomic: store of inconsistently typed value into Value
```

位置：[egresshealth.go](../../internal/relay/egresshealth.go)。main 使用 `atomic.Value` 存放不同具体类型的 Dialer，v4.5.6 已改为统一 holder。API handler 会恢复部分 panic，但热应用过程可能中断；不能因为 CLI 输出成功就判为正常。

证据：[main 流程测试](linux-audit-2026-11-06/main-smoke.txt)、[v4.5.6 对照测试](linux-audit-2026-11-06/release-smoke.txt)。

### 4. Linux 自动化测试无法完整编译，发布流水线未运行 Linux 测试

[firewall_test.go](../../internal/firewall/firewall_test.go) 和第 271 行在公共测试文件中引用 `darwinManager`，该类型仅在 darwin 编译。main 与 v4.5.6 都存在。

[发布工作流](../../.github/workflows/release.yml) 仅在 macOS runner 上测试，再交叉编译 Linux 安装包，因此能发布 Linux 主程序，但漏掉 Linux 测试编译和平台运行缺陷。

## 本地复跑

工具整理入库后再次验证：`smoke` 28 项通过；`udp-transition` 与 `transparent` 均复现上述 UDP 故障并返回非零退出码；临时容器和独立测试网络均已自动清理。归档证据保留最初实验输出，堆栈中的脚本行号对应整理前版本，本机用户目录已脱敏。

使用 [Docker 测试工具](../../tools/linux-audit/README.md)。工具接收已有的 Linux 可执行文件，并在临时容器中分别运行 CLI 冒烟测试、UDP 模式切换检查、双容器透明转发检查。已知故障会返回非零退出码，不作为测试成功。

建议处理顺序：统一 main 与发布版本代码基线；修复 Linux UDP 目标恢复和 UDP 服务热切换；拆分平台专属测试并加入 Linux CI；最后补齐 CLI 的线性引导和高级功能入口。systemd、真实设备、非 Alpine 发行版兼容性和升级回滚仍需独立验证。
