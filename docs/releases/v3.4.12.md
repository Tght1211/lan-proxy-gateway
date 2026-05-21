# LAN Proxy Gateway v3.4.12

**WebUI 访问与分组规则增强版**。这一版修复 Gateway WebUI token 不一致导致反复跳回未授权页的问题，并让自定义规则可以指定具体 mihomo 策略组。

## 新增

- **自定义规则可指定策略组**

  之前自定义规则只有三类去向：`DIRECT`、`Proxy`、`REJECT`。现在新增「指定分组」，规则可以直接路由到任意 mihomo policy group，例如：

  - `DOMAIN-SUFFIX,openai.com` → `🛬 AI落地节点`
  - `DOMAIN-SUFFIX,anthropic.com` → `🛬 AI落地节点`
  - YouTube / Netflix / GitHub 等普通全局流量继续走默认 `Proxy`

  这样链式住宅 IP 不再需要覆盖所有代理流量，只给 AI 相关域名使用住宅出口。

- **CLI 和 WebUI 同步支持**

  CLI 的自定义规则菜单新增「指定策略组」去向，会列出当前 mihomo 策略组并支持手动输入；WebUI 的自定义规则卡新增「指定分组」页签，可下拉选择当前策略组或手动输入组名。

- **README 新增 WebUI 截图**

  README 现在展示新的 Gateway WebUI 自定义规则交互，方便用户理解 `19091` 控制台能做什么。

## 修复

- **WebUI token 不再每个入口各生成一个**

  旧配置缺少 `runtime.web_ui_token` 时，`gateway start`、WebUI daemon、`sudo gateway` 菜单可能各自在内存里生成不同 token，导致启动横幅里的 URL 和菜单里的 URL 不一致，浏览器访问后又被提示没有 token。

  现在缺失 token 会在加载配置时生成一次并写回 `gateway.yaml`，之后所有入口读取同一个 token。

- **旧 WebUI daemon token 不匹配时会重启**

  `gateway start` 现在用当前 token 请求 `/api/status` 探测 WebUI daemon。如果 daemon 还活着但 token 不一致，会先停止旧 daemon 再启动新的，避免复用旧进程。

- **本机访问 WebUI 不强制 token**

  `http://127.0.0.1:19091/` / `http://localhost:19091/` 的 API 请求现在直接放行，方便在 gateway 主机上操作；局域网 `http://<LAN IP>:19091/` 仍然需要 CLI 打印的 `#token=...` URL。

## 测试

- `node --check internal/webui/static/app.js`
- `go test ./...`
- `make build-all VERSION=v3.4.12`
- Playwright/Chrome 截图验证 WebUI 自定义规则「指定分组」页面

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.12/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.12/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.12/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.12/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.12/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.12/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.12/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.12/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.12/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.12/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.12/SHA256SUMS)

## 升级 / 安装

```bash
gateway update v3.4.12
```

如果当前版本太旧没有 `gateway update`，用安装脚本覆盖安装，会保留 `gateway.yaml`：

```bash
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | bash
```
