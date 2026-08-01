# LAN Proxy Gateway 文档

[返回项目首页](../README.md)

## 第一次使用

1. 按 [设备接入总览](device-setup.md) 理解需要填写的网络参数。
2. 根据设备选择教程：[手机 / 平板](phone-setup.md)、[Nintendo Switch](switch-setup.md)、[PS5](ps5-setup.md)、[Apple TV](appletv-setup.md) 或 [智能电视](tv-setup.md)。
3. 遇到问题先看 [常见问题](faq.md)，再用 `gateway status` 和运行日志定位。

也可以把 [AI 配置提示词](ai-setup.md) 交给能够操作本机终端的 Codex、Claude Code 等工具，完成电脑端的检查、安装和启动。

## 使用指南

| 文档 | 内容 |
|---|---|
| [macOS App](app.md) | 页面功能、数据与隐私、安装和本机构建 |
| [命令说明](commands.md) | 生命周期、系统代理、开机自启和更新命令 |
| [配置文件](advanced.md) | `gateway.yaml`、fake-IP 缓存和常见上游 |
| [典型场景](scenarios.md) | 同机代理、直连网关、远程上游和排错清单 |
| [实机结果](real-device-results.md) | 手机与 Switch 的实际测速和访问效果 |

## 项目说明

| 文档 | 内容 |
|---|---|
| [架构](architecture.md) | DNS、透明 TCP relay、防火墙和系统代理 |
| [从 v3 升级](migration-v4.md) | 破坏性变化、备份与迁移步骤 |
| [Changelog](../CHANGELOG.md) | 各版本新增功能与修复 |
| [示例配置](../gateway.example.yaml) | 完整的 v4 配置字段 |

## 支持范围

- 宿主系统：macOS、Linux（amd64 / arm64）。
- 透明转发：IPv4 TCP；HTTP CONNECT 或 SOCKS5 上游。
- 平台网络：macOS `pf`、Linux `iptables`。
- 不支持：Windows、Docker、普通路由器/OpenWrt、IPv6 透明代理、通用 UDP 代理和 TUN 模式。

项目不提供代理节点、订阅或流媒体解锁。复杂节点选择与规则集应继续交给 Clash、Mihomo 或 sing-box。
