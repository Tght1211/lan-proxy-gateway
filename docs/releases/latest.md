# lan-proxy-gateway v4.0.3

这是 v4 重构后的稳定版本，面向常驻 Mac mini 和 Linux 小主机：局域网设备只需把网关与 DNS 指向宿主机，即可复用现有 Clash、Mihomo、sing-box 或其他 HTTP/SOCKS5 代理软件。

## 本次修复

- 修复 HTTP CONNECT 重复发送 `Host` 请求头的问题。部分 mixed 代理端口此前会直接关闭连接，日志表现为 `unexpected EOF`，导致局域网设备无法访问互联网。
- 修复标准 `200 Connection established` 成功响应没有消息体时，gateway 错误关闭响应对象并连带关闭代理隧道的问题。
- HTTP 代理出口已使用 PigLite mixed 端口完成真实 TLS 握手和局域网设备验证。
- 保留 v4.0.2 的长连接修复：YouTube 等视频连接不会再被固定两分钟超时强制中断。
- 保留 Go 优化的 TCP 数据复制路径，Linux 可继续使用零拷贝 `splice`。

## 文档与实际设备验证

- README 新增 PC 直连代理、手机经过 gateway 的 Fast.com 对比截图。
- 新增 Nintendo Switch 网络测试与通过旁路由访问 YouTube 的实机截图。
- 修正手机与 PlayStation 配置文档中的 DNS、NAT 和分流说明。
- 明确 gateway 与第三方代理均不需要开启 TUN。
- gateway 不提供节点、订阅、流媒体解锁或分流规则；YouTube、Googlevideo 等流量最终走哪个节点，由第三方代理软件决定。

## 升级说明

### 从 v4.0.0 / v4.0.1 / v4.0.2 升级

可直接执行：

```bash
sudo gateway update v4.0.3
```

现有 v4 配置可以继续使用。升级后建议执行 `sudo gateway restart`，确保旧进程和长连接全部切换到新版本。

### 从 v3 升级

v4 是完整重构，不兼容 v3 配置。更新程序会明确提示并要求确认：

- 内置 mihomo、订阅、节点、规则集、WebUI、流量面板、脚本和 Windows 构建均已移除。
- 旧配置会备份为 `gateway.yaml.pre-v4.bak*`，之后需要重新完成 v4 初始化。
- 节点与分流规则必须迁移到独立运行的第三方代理软件。
- 更新前请备份 `~/.config/lan-proxy-gateway/`，并记下 HTTP/SOCKS5 代理地址与端口。

## 支持范围

- macOS amd64 / arm64
- Linux amd64 / arm64
- IPv4 TCP 透明转发
- HTTP CONNECT / SOCKS5 上游
- macOS `pf` / Linux `iptables`

暂不支持 Windows、Docker、普通路由器/OpenWrt、IPv6 透明转发、UDP 代理或 TUN 模式。代理模式会拒绝 UDP/443，使客户端从 QUIC 快速回退到可代理的 TCP；其他 UDP 保持直连。
