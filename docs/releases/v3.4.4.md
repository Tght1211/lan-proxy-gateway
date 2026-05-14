# LAN Proxy Gateway v3.4.4

**Docker + 局域网 hotfix**。修掉 issue [#5](https://github.com/Tght1211/lan-proxy-gateway/issues/5) 真正的根因：`gateway stop` 之前会无脑把 `net.ipv4.ip_forward` 写回 `0`，撞断了那些 gateway start 之前就靠 ip_forward=1 暴露端口给局域网的 docker 服务（比如 @lingbaoboy 的 immich 2228 端口）。

## 修复

- **`gateway stop` 不再误关 docker 的 LAN 暴露**

  之前每次 stop 都把 `/proc/sys/net/ipv4/ip_forward` 打回 0。但 docker 把容器端口发布给局域网（`-p host:container` + 跨 interface DNAT）也依赖 ip_forward=1。原本就是 1 的环境（docker 启动时自己开的、systemd-sysctl 配的、用户手动开的）被 gateway stop 撞断，局域网立刻访问不到 host 上的服务。

  v3.4.4 起，`gateway start` 把改 ip_forward 前的状态写到 `~/.config/lan-proxy-gateway/runtime.state`，只有当我们真的把它从 0 改成 1 时，`gateway stop` 才回退到 0；原本就是 1 的保持 1 不动。多次 `gateway start` / 重启进程也保留这个 flag，不会因为现在的值是 1 就忘记自己改过。

- **`gateway stop` 现在真的会删 NAT MASQUERADE 残留**

  之前 `Disable()` 在 `g.info.Interface == ""` 时直接跳过 `UnconfigureNAT`。`gateway stop` 是独立进程，没经过 `Enable()` 那条路 → 内存里 `info` 是空的 → MASQUERADE 永远删不掉。现在优先用 state 文件里记下的 `NATInterface`，没 state 文件时降级到现场 `Detect()`，再不济也试着把残留删了。

- **state 文件丢了也安全**

  升级前没有 state 文件的存量用户：`gateway stop` 不会动 ip_forward（我们不知道是不是自己开的，默认不动 = docker 友好），但还是会用 Detect() 反删 MASQUERADE，并继续跑 v3.4.2 的 ip rule 清理。

## 验证（用户复现路径）

issue #5 报告者 @lingbaoboy 的复现：

```bash
docker compose up -d immich   # 把 2228 暴露给局域网
sudo gateway start            # 拿出去
# 局域网设备访问 http://10.0.0.11:2228 → 200 ✅
sudo gateway stop
# 局域网设备再访问一次 → v3.4.4 起仍然 200 ✅（v3.4.3 之前会超时）
```

## 保留的既有修复

- **v3.4.3** 继续保留 `gateway update [version]`、`gateway stop` 时恢复本机 DNS。
- **v3.4.2** 继续保留 mihomo v1.19.24、anytls 支持、SIGTERM 8s grace、Linux `PostStopCleanup()` 清理残留 TUN strict-route ip rule。
- **v3.4.1** 继续保留 systemd `network-online.target` 修复，以及 `traffic.auto_groups` 注入 Auto / Fallback / DIRECT。
- **v3.3.2** 继续保留 IPv6 / DNS AAAA 默认开启。

## 测试

- `go test ./...`
- `gofmt -l ./...`、`go vet ./...`
- 新增 `internal/gateway/state.go` + `internal/gateway/gateway_test.go` 6 个用例覆盖：
  - 启动前 ip_forward=1，stop 必须保留（issue #5 主因）
  - 启动前 ip_forward=0，stop 必须回退到 0
  - 独立进程 stop 仍能从 state 文件读到 NATInterface 反删 MASQUERADE
  - 没 state 文件时降级到 Detect() 反删，但不动 ip_forward（安全默认）
  - PostStopCleanup 始终会跑
  - 多次 Enable() 幂等：第二次不会清掉 WeEnabledIPForward flag

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.4/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.4/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.4/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.4/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.4/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.4/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.4/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.4/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.4/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.4/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.4/SHA256SUMS)

## 升级

```bash
sudo gateway update v3.4.4
```

或者全新安装：

```bash
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | sudo bash
```
