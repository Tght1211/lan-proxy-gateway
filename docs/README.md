# LAN Proxy Gateway 文档

[返回项目首页](../README.md)

**第一次使用只需要一份文档：[完整图文使用手册](user-guide.md)。** 按安装、代理配置、设备接入、验证的顺序走，不必先读命令行或架构。

## 从这里开始

| 你的问题 | 文档 |
|---|---|
| Release 附件太多，不知道选哪个 | [下载与安装包对照表](download.md) |
| 刚装好，想接通第一台设备 | [完整使用手册](user-guide.md) |
| Mac 有网线，想给手机 / 游戏机开热点 | [热点路线](user-guide.md#wifi) |
| 不想建热点，只给浏览器或手机配置代理 | [HTTP 代理](user-guide.md#http) / [PAC](user-guide.md#pac) |
| 想在原来的网络里设置静态网关 | [静态网关路线](user-guide.md#static) |
| 已连接但打不开网页 / 商店 / 游戏 | [常见问题](faq.md) |
| 想更新、停用或卸载 | [更新](user-guide.md#update) / [停止与恢复](user-guide.md#stop) |
| Linux 或纯命令行用户 | [CLI 安装](download.md#linux) |

## 按设备查设置菜单

这些页面补充设备端的菜单路径。先在主手册选好一种接入方式，再看对应设备；不要同时套用热点自动获取和静态地址教程。

[手机 / 平板](phone-setup.md) · [Switch](switch-setup.md) · [PS5](ps5-setup.md) · [Apple TV](appletv-setup.md) · [电视 / 盒子](tv-setup.md)

## 进阶与参考

| 文档 | 用途 |
|---|---|
| [App 功能参考](app.md) | 页面职责、统计口径、运行机制和本机构建 |
| [热点详细说明](hotspot-setup.md) | 信道建议、系统共享、检测与接管边界 |
| [设备接入对照](device-setup.md) | 三类接入的参数差别 |
| [命令行](commands.md) | 状态、启停、出口、系统服务和更新 |
| [外部 Agent Skill](agent-skill.md) | 导出、安装和使用本机 AI Agent |
| [配置文件](advanced.md) | `gateway.yaml` 和高级设置 |
| [架构](architecture.md) | DNS、转发、防火墙与控制面 |
| [从 v3 升级](migration-v4.md) | 历史版本迁移 |
| [Linux CLI 验证](testing/linux-audit-2026-11-06.md) | 2026-11-06 Docker 实测、功能差异和已知缺陷 |
| [实机结果](real-device-results.md) | 历史测试记录，不作为当前网络效果保证 |
| [更新记录](../CHANGELOG.md) | 源码中的版本记录；已发布版本以 Release 为准 |

软件运行在 macOS / 完整 Linux 上。它共享已有 HTTP/SOCKS5 代理，不提供节点、订阅或通用 UDP 代理。图形 App 要求 macOS 13+；完整附件适用范围见 [下载表](download.md)。
