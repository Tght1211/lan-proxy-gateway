# 命令说明

## 控制面板

`sudo gateway` 不带参数时打开静态控制面板。启停旁路由和修改 macOS 系统代理需要管理员权限：

```text
1  启动/停止旁路由
2  设置代理（SOCKS5 / HTTP / 直连）
3  查看设备参数
4  查看最近日志
Q  退出
```

面板只有一层菜单：生命周期按当前状态直接启停，日志只显示最近 40 行。它不自动刷新，也不包含流量图、设备列表、连接表或 DNS 高级设置。

## 旁路由生命周期

```bash
sudo gateway install             # 初始化、启动、可选安装系统服务
sudo gateway start               # 启动后台守护进程
sudo gateway start --foreground  # 前台运行
sudo gateway restart
sudo gateway stop
gateway status [--json]
```

守护进程日志位于 `~/.config/lan-proxy-gateway/gateway.log`。启动需要管理员权限，因为程序需要启用 IPv4 转发、监听 53 端口并配置 pf/iptables。

## macOS 系统代理

```bash
gateway system-proxy status [--json]
gateway system-proxy on --type socks5 --host 127.0.0.1 --port 7897
gateway system-proxy on --type http --host 127.0.0.1 --port 7897
gateway system-proxy off
```

这些命令通过 `networksetup` 修改 macOS 网络服务。HTTP 模式同时设置 HTTP 和 HTTPS 代理；启用 HTTP 或 SOCKS5 时会关闭另一种模式，并把同一地址同步为旁路由上游。执行 `off` 时旁路由切回直连。

是否能访问特定外网主要取决于该地址背后的代理服务和节点。gateway 支持域名 / IP-CIDR 的轻量分流，但不负责订阅、节点选择或复杂规则集。

## 分流规则

```bash
gateway routing list [--json]
gateway routing set --rules-json '[{"type":"domain-suffix","value":"example.com","action":"direct"}]'
```

规则按顺序首次命中，类型支持 `domain`、`domain-suffix`、`ip-cidr`，动作支持 `proxy`、`direct`、`reject`。macOS App 提供可视化编辑器，日常使用不必手写 JSON。

## 开机自启

```bash
sudo gateway service install
sudo gateway service uninstall
gateway service status
```

macOS 使用 launchd，Linux 使用 systemd。

## 更新与重构迁移

```bash
gateway update              # 推荐：下载阶段保留当前用户的代理环境
sudo gateway update         # 也可用，但 sudo 可能清除 HTTP_PROXY/HTTPS_PROXY
gateway update --yes        # 自动化场景，确认并跳过迁移询问
```

当前版本是完整重构。更新命令会先明确提示：mihomo、订阅、节点、规则集、WebUI 和旧控制台已移除；旧配置会备份，升级后需要重新初始化。未确认时不会下载或替换二进制。
