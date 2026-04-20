# LAN Proxy Gateway v3.0.6

体验版：主菜单 5「日志」加了**易读视图**，把 mihomo 的英文技术日志翻译成中文简要，排问题不用再背术语。

## 重点变更

### 日志菜单默认易读视图，`r` 可切回原始

```
sudo gateway → 5 日志
```

默认显示翻译后的简要版；按 `r` 在「易读 / 原始」之间切换。`t` 仍然是 tail 跟随。

### 翻译效果举例

原始行：
```
time="2026-04-21T01:27:55.642635000+08:00" level=warning \
  msg="[TCP] dial DIRECT (match GeoIP/cn) 198.18.0.1:50931 \
       --> 115.190.130.195:21114 error: connect failed: dial tcp ...: i/o timeout"
```

易读视图：
```
🟡 01:27:55  TCP 直连 115.190.130.195:21114 （GeoIP=cn）  → 目标无响应（超时）
```

### 识别规则

- **路由类型**：`DIRECT` → `直连` / `Proxy` / `具体节点` → `走代理[名字]` / `REJECT` → `拒绝`
- **匹配规则**：`GeoIP/cn` → `GeoIP=cn` / `Match/` → `兜底` / `DomainSuffix/x` → `域名=x` / `DomainKeyword/x` / `Domain/x` / `IPCIDR/x` / `ProcessName/x`
- **常见错误**：
  - `127.0.0.1:xxxx connection refused` → **本机代理拒绝连接（检查代理软件是否启动）**
  - `connection refused` → 目标主动拒绝（端口上没服务）
  - `i/o timeout` → 目标无响应（超时）
  - `resource temporarily unavailable` → 本机 socket 资源暂时不足（重启 mihomo 清一下）
  - `can't resolve ip` 含 `1.1.1.1` / `8.8.8.8` → **域名解析失败（fallback DoH 没走代理？）**
  - `network is unreachable` / `no route to host` / `context deadline exceeded` / `EOF` 都有对应中文

- **等级图标**：🔴 error / 🟡 warning / ℹ️ info

识别不了的行**原样返回**，信息不丢。

### 「网关状态 / 设备接入指引」文案精简

之前的指引有 10+ 行、中间夹 TUN / DNS 细节、结尾还有"YouTube 能连但 Switch 连不上就重启"等零散提示，小白直接懵。本版改成：

- 顶部只列**本机 IP + 路由器 IP** 两个数字
- 中间四行填写项：**网关 / DNS / 子网掩码 / IP 地址**，每一项都写清楚填什么（子网掩码之前写的是"保持不变"，现在明写 `255.255.255.0` 并说明"99% 家庭网就这个值"）
- 底部一行前提：TUN 和 DNS 代理都要开

原来的 TUN 关闭警告、DNS 代理关闭警告已经在主菜单顶部状态栏有显眼提示，不用再在这里重复。

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.6/gateway-darwin-arm64) / [gateway-darwin-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.6/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.6/gateway-darwin-amd64) / [gateway-darwin-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.6/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.6/gateway-linux-amd64) / [gateway-linux-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.6/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.6/gateway-linux-arm64) / [gateway-linux-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.6/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.6/gateway-windows-amd64.exe) / [gateway-windows-amd64.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.6/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.6/SHA256SUMS)
