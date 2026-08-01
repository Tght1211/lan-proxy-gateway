# 架构

项目管理 LAN 旁路由和 macOS 系统代理。两者复用同一个 HTTP/SOCKS5 代理地址。gateway 提供少量有序分流规则；订阅、节点选择和复杂规则集仍由外部代理软件负责。

部署目标是常驻的 Mac mini 或完整 Linux 小主机，不是普通路由器/OpenWrt，也不内置第三方代理软件。

## LAN 旁路由

```text
LAN 设备（网关和 DNS 指向本机）
  -> pf / iptables 捕获转发进来的 TCP
  -> relay 恢复原始目标
  -> direct / SOCKS5 / HTTP CONNECT dialer

LAN DNS 查询 -> 内置 DNS 转发器 -> 上游 DNS
```

- macOS 使用 pf `rdr` 与 `DIOCNATLOOK`。
- Linux 使用 iptables `REDIRECT` 与 `SO_ORIGINAL_DST`。
- 单接口 macOS 即使选择直连，也让 TCP 经过 relay，避免同接口 pf NAT 无法正确回包。
- 代理模式阻断 UDP/443，让浏览器从 QUIC 回退到可代理的 TCP。
- 代理模式默认启用 fake-IP，把原始域名交给上游代理解析和分流；默认不劫持设备主动发往其他 DNS 的查询。
- 有序规则支持 `domain`、`domain-suffix`、`ip-cidr` 匹配，并选择 `proxy`、`direct` 或 `reject`。没有显式规则时使用当前出口。
- 默认代理拨号失败而直连成功的域名可以进入回退学习，并在达到阈值后生成可删除的直连规则。

`gateway start` 启动脱离终端的 `gateway run` 守护进程。守护进程负责防火墙、DNS、relay、配置热加载和本机回环状态 API。`gateway stop` 会拆除规则并恢复由本程序开启的 IP 转发/pf 状态。

原生 macOS App 通过本机回环状态 API 读取流量、设备、服务、网络质量和连接结果。连接历史只保存在守护进程内存中，不是远程 WebUI。

## macOS 系统代理

`internal/systemproxy` 调用 `networksetup`：

- SOCKS5：设置并启用 SOCKS Firewall Proxy，关闭 HTTP/HTTPS Web Proxy。
- HTTP：设置并启用 HTTP 与 HTTPS Web Proxy，关闭 SOCKS Firewall Proxy。
- 关闭：关闭三种代理状态。

这部分不依赖旁路由守护进程，不修改 DNS，也不需要设备接入旁路由。

## 目录

```text
cmd/                  命令行入口
internal/app/         旁路由生命周期与守护进程
internal/config/      配置加载、保存、校验
internal/console/     静态终端控制面板
internal/dns/         LAN DNS 服务
internal/firewall/    pf / iptables 规则
internal/gateway/     网络探测和启停编排
internal/platform/    平台差异
internal/relay/       透明 TCP relay 与出口 dialer
internal/systemproxy/ macOS 系统代理
```

## 不在范围内

代理订阅、节点管理、复杂规则集、远程 WebUI、TUN、IPv6 透明代理和通用 UDP 代理均不在项目范围内。
