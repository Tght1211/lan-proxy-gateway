# LAN Proxy Gateway v3.0.5

性能修复版：修掉了国内用户解析国外域名奇慢 / 超时的问题。

## 修复

### fallback DoH 现在走代理组

mihomo 的 DNS 模板里 fallback 用的是 DoH：

```yaml
fallback:
  - https://1.1.1.1/dns-query
  - https://8.8.8.8/dns-query
```

之前没有显式 `#Proxy` 标记，mihomo 默认**直连** 1.1.1.1:443 / 8.8.8.8:443 发 DoH。国内用户直连这俩 IP 基本是超时的，于是：

- `cn.pool.ntp.org` 等需要走 fallback 的域名解析永远超时
- `translate-pa.googleapis.com`、`googleapis.com` 等国外域名解析反复失败
- 前台表现：网页打开慢、Google / 翻译打不开、节点延迟探测也被拖慢

v3.0.5 给 fallback 加 `#Proxy` 让 DoH 查询走 `Proxy` 代理组，代理到 Cloudflare / Google 后 DoH 就能正常应答了。

### 不是问题的：`DIRECT i/o timeout`

日志里这类行：
```
[TCP] dial DIRECT (match GeoIP/cn) xxx --> 115.190.130.195:21114 error: connect failed
```

是 LAN 里某台设备有后台程序在反复连一个已挂 / 拒绝的国内服务器，命中 `GeoIP/cn` 规则走 DIRECT。跟 gateway / 代理都无关，本版**不处理**。

## 升级后如何立即生效

v3.0.5 的 template 更新只在**下次启动或热重载**时生效。升级完之后任选一种让新配置上线：

- `sudo gateway`（进主菜单）→ 选 `2 流量控制` → `1 切换模式` → 随便选一个模式再切回来（触发热重载）
- 或 `sudo gateway stop && sudo gateway start`

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.5/gateway-darwin-arm64) / [gateway-darwin-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.5/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.5/gateway-darwin-amd64) / [gateway-darwin-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.5/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.5/gateway-linux-amd64) / [gateway-linux-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.5/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.5/gateway-linux-arm64) / [gateway-linux-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.5/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.5/gateway-windows-amd64.exe) / [gateway-windows-amd64.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.5/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.0.5/SHA256SUMS)
