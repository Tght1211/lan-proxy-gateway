# 从 v3 升级到 v4

[返回文档索引](README.md)

v4 是完整重构，不兼容 v3 配置。它移除了内置 mihomo、订阅、节点、旧规则集、WebUI、脚本和 Windows 构建；这些能力应迁移到独立运行的 Clash、Mihomo 或 sing-box。

## 升级前

1. 备份 `~/.config/lan-proxy-gateway/`。
2. 记下第三方代理的协议、地址和端口。
3. 确认代理软件能够独立运行，并提供 HTTP 或 SOCKS5 监听端口。

## 执行升级

```bash
gateway update
```

更新程序会显示迁移说明并要求确认。未确认时不会替换当前二进制。旧配置会备份为 `gateway.yaml.pre-v4.bak*`，首次运行 v4 时需要重新初始化。

建议直接以当前用户运行 `gateway update`。程序需要替换系统路径中的二进制时会自行请求 `sudo`，同时保留当前用户的代理环境变量。

## 升级后

```bash
sudo gateway install
gateway status
```

重新配置 HTTP/SOCKS5 上游，并按 `gateway status` 的输出核对设备网关与 DNS。v3 的订阅、节点和复杂规则不会自动转换为 v4 配置。

同一主版本内的更新通常可以直接执行 `gateway update`。具体变化见 [Changelog](../CHANGELOG.md)。
