# LAN Proxy Gateway v3.4.9

**`gateway update` 代理穿透修复版**。如果你曾在 HTTPS 代理下跑 `gateway update` 看到一堆 `EOF` 或 `malformed HTTP response "\x00\x00\x12\x04..."`，这版彻底修好了那条路径。

## 修复

- **`gateway update` 在 HTTPS 代理下不再 EOF**

  `cmd/update.go` 里 `tr.ForceAttemptHTTP2 = false` 原本想禁用 HTTP/2，但 `http.DefaultTransport.(*http.Transport).Clone()` 之后 `TLSNextProto` 是空 map（非 nil），TLS ALPN 仍会协商成 h2，结果传输层握手成 h2、应用层却按 h1 读响应。所有 GitHub release URL 走代理都拿到 `EOF`，最后一个镜像源直接被 HTTP/2 SETTINGS 帧打成 `malformed HTTP response`。删掉这行，让 Go transport 默认处理 HTTP/2。

- **`sudo gateway update` 会先在用户身份下下载，再 elevate 安装**

  以前 `gateway update` 一进来就 `maybeElevate()`，整个流程在 root 身份跑。macOS sudoers 默认 `env_reset` 会清掉 `HTTPS_PROXY` / `HTTP_PROXY`（即便 `--preserve-env=` 也会被默认策略静默挡掉），root 进程没代理变量，下载/查版本必失败。

  重构后的流程：

  1. **用户身份阶段**：版本查询 + 下载到 `/tmp` —— 代理变量完整可用。
  2. **切换到 sudo**：通过隐藏 flag `--prefetched-tag` / `--prefetched-asset` 把下载好的临时文件路径透传给 root 子进程。
  3. **Root 阶段**：仅 stop / 替换二进制 / restart，不再需要联网。

  对应推荐写法是 `gateway update`（不带 sudo），程序会在需要 root 时自己请求 sudo 并保留你的代理设置。如果你已经习惯 `sudo gateway update`，命令仍能跑，但代理变量缺失时会打一条黄色提示，建议改用不带 sudo 的方式。

- **`gateway update` 拉 latest 时新增 release 页面跳转回退**

  GitHub API `/releases/latest` 即便走代理也偶尔会被节流到 5xx 或被某些镜像源穿透成乱码。新加一条回退：API 全部失败时改抓 `https://github.com/<repo>/releases/latest`，沿着 302 取最终 URL 里的 tag。

## 内部改进

- **`cmd/update.go` 重构**：提取 `prepareUpdateBinary` / `stopGatewayBeforeUpdate` / `restartGatewayAfterUpdate` 三个 helper + 三个字符串常量（`updateUserAgentHeader` / `updateUserAgentValue` / `updateErrCandidateFmt`），消化掉 SonarQube 之前的 cognitive-complexity 和重复字面量警告。纯重构，无行为变化。
- **`legacy/` 标注归档**：新加 `legacy/README.md` 说明这是历史归档（`legacy/v1/` 是独立 Go module、shell 脚本预 Go 时代），不维护、不引入、不外链。
- **辅助脚本统一到 scripts/**：`install-mihomo.sh` / `download-mihomo.sh` 移入 `scripts/`，根目录保留 4 行 wrapper 兼容旧调用（`bash install-mihomo.sh` 依然可用）；`script-demo.js` 改名为 `examples/extension.js`，`docs/advanced.md` 与 `docs/en/advanced.md` 同步更新。`install.sh` / `install.ps1`（README 与多版本 release notes 引用的公开 install URL）和 `dev.sh` / `dev.ps1`（README_EN 文档化的开发入口）保持在根目录不动。
- **`make clean` 扩容**：除 `dist/`、`gateway` 之外，还清 `.tmp/`、`.cache/`、`.try/`、`logs/*.log` 和散落的 `.DS_Store`；不会动 `handoff_unfinished_tasks.txt` 这类人工接力文档。

## 测试

- `go test ./...`
- 手动验证 `HTTPS_PROXY=http://127.0.0.1:7897 gateway update latest` 能正确通过代理拿到 latest tag 并完成下载。
- `bash -n install-mihomo.sh && bash -n scripts/install-mihomo.sh && bash -n download-mihomo.sh && bash -n scripts/download-mihomo.sh` 全部通过；wrapper 实测能 exec 到目标。

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.9/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.9/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.9/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.9/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.9/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.9/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.9/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.9/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.9/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.9/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.9/SHA256SUMS)

## 升级

```bash
gateway update
```

**注意**：本版恰好修了"`sudo gateway update` 在 macOS 默认 sudoers 下丢失代理变量"的问题，但推荐写法是**不预先 sudo**，让程序自己决定何时切换 sudo —— 这样代理变量保留得最干净。

或指定版本：

```bash
gateway update v3.4.9
```

如果你卡在 v3.4.8 以前的版本，升级 `gateway update` 自身的修复需要先一次"重装"才会生效：

```bash
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | sudo bash
```

之后再用的 `gateway update` 就走新流程了。
