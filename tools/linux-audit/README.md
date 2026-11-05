# Linux CLI Docker 验证

这是开发者测试工具。测试结果和 Desktop 功能对照见 [2026-11-06 审计报告](../../docs/testing/linux-audit-2026-11-06.md)。工具记录已知失败，尚未修复网关功能。

需要已运行的 Docker、宿主机 Python 3 和一个 Linux 可执行文件。可执行文件必须匹配 Docker 引擎架构；例如本次 Docker Desktop 使用 Linux ARM64。发布安装包应先与同一 Release 的 `SHA256SUMS` 比对。不要使用 macOS 二进制。

## 运行

从项目根目录执行，替换文件路径：

```bash
python3 tools/linux-audit/run.py /absolute/path/gateway-linux-arm64 smoke
python3 tools/linux-audit/run.py /absolute/path/gateway-linux-arm64 udp-transition
python3 tools/linux-audit/run.py /absolute/path/gateway-linux-arm64 transparent
```

也可以先构建当前代码（`GOARCH` 应匹配 Docker 引擎）：

```bash
mkdir -p .tmp/linux-audit
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o .tmp/linux-audit/gateway-linux-arm64 .
python3 tools/linux-audit/run.py .tmp/linux-audit/gateway-linux-arm64 smoke
```

| 测试 | 检查内容 | v4.5.6 本次结果 |
| --- | --- | --- |
| `smoke` | 初始化、状态、真实 iptables 与 IP 转发、HTTP、CONNECT、PAC、认证、HTTP/SOCKS5 上游、规则热应用、DNS、学习操作、重启和停止清理 | 28 项通过 |
| `udp-transition` | 默认直连启动后切到代理，UDP 监听器应立即存在；打印重启后的对照状态 | 失败：重启前监听器不存在 |
| `transparent` | 独立客户端通过网关 DNS 和 fake-IP 路由发起 TCP、UDP 请求 | DNS/TCP 通过；UDP 回包超时 |

失败返回非零退出码。报告受测 main 的 `smoke` 还会因为 HTTP → SOCKS5 切换产生 panic 而失败；这不等于测试脚本不能运行。排查应同时看 CLI 返回值、实际请求结果和核心日志。

## 隔离与边界

运行器构建测试镜像，创建唯一名称的临时 Docker 网络和容器，只读挂载指定的可执行文件。网关容器需要 `NET_ADMIN` 和 `systempaths=unconfined`，以修改其自身网络命名空间的 iptables、路由与 `ip_forward`。工具不发布宿主机端口、不挂载宿主机系统目录，也不使用 host 网络。

测试必须通过运行器执行。容器脚本检查镜像标记和 `/.dockerenv`，避免被误用于真实主机。结束、断言失败和通常的 Ctrl-C 都会清理本次资源；测试镜像保留以便复跑。进程被强制杀死时可按本次 `lpg-audit-*` 名称检查遗留容器和网络。

HTTP 和 SOCKS5 上游均为本地模拟服务，接收到的请求送往本地 HTTP 源站。透明测试的 UDP 源站也是本地 echo 服务。通过不代表公网代理质量、设备兼容性、systemd 开机自启、更新回滚或所有 Linux 发行版已通过验收。

`origdst-probe.py` 是排查 UDP 时使用的辅助接收器，监听 18883 并将控制消息写到容器 `/tmp/origdst-result.json`；它不在三个常规测试中自动运行。原始实验的 REDIRECT 规则和抓取结果记录在审计报告中。
