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

DNS、防火墙和运行端口属于旁路由内部配置。推荐默认值为 `fake_ip: true`、`hijack: false`：代理模式保留原始域名给 Clash/sing-box 解析和分流，但不拦截设备主动发往其他 DNS 的查询。

fake-IP 与原始域名的对应关系保存在 `~/.config/lan-proxy-gateway/fakeip-cache.json`，用于在 gateway 重启后继续识别设备已缓存的虚拟地址。缓存文件权限为 `0600`，默认保留 7 天并受容量上限约束；写入使用原子替换。该文件包含设备访问过的域名，不要公开分享。通常不需要手动处理；删除后应让局域网设备重新连接 Wi-Fi，以便重新解析 DNS。

常见代理软件入口：

| 上游 | 类型 | 地址 |
|---|---|---|
| Clash Verge / Mihomo | HTTP 或 SOCKS5 | `127.0.0.1:7897` |
| sing-box | 按本地入站类型 | 其本地监听端口 |
| `ssh -D 1080 host` | SOCKS5 | `127.0.0.1:1080` |

运行日志位于 `~/.config/lan-proxy-gateway/gateway.log`。53 或 19090 端口冲突时，启动预检会报告占用进程。
