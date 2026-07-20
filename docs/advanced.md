# 配置文件

配置文件位于 `~/.config/lan-proxy-gateway/gateway.yaml`。通常应通过控制面板或 `gateway system-proxy` 修改；守护进程会自动加载变更。

完整示例见 [`gateway.example.yaml`](../gateway.example.yaml)。macOS 用户通常无需手动编辑出口；`gateway system-proxy on|off` 会同步它：

```yaml
egress:
  mode: proxy             # direct | proxy
  proxy:
    type: socks5          # socks5 | http
    host: 127.0.0.1
    port: 7897
    username: ""
    password: ""
```

DNS、防火墙和运行端口属于旁路由内部配置。推荐默认值为 `fake_ip: false`、`hijack: false`，即返回真实 IP，且不拦截设备主动发往其他 DNS 的查询。

常见代理软件入口：

| 上游 | 类型 | 地址 |
|---|---|---|
| Clash Verge / Mihomo | HTTP 或 SOCKS5 | `127.0.0.1:7897` |
| sing-box | 按本地入站类型 | 其本地监听端口 |
| `ssh -D 1080 host` | SOCKS5 | `127.0.0.1:1080` |

运行日志位于 `~/.config/lan-proxy-gateway/gateway.log`。53 或 19090 端口冲突时，启动预检会报告占用进程。
