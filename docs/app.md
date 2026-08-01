# macOS App

[返回文档索引](README.md)

原生 SwiftUI App 和 Go CLI 连接同一个 `gateway` 守护进程，共用配置、状态 API、日志和网络规则。服务已经运行时，从另一个前端启动只会连接现有服务，不会创建第二个数据面进程。

## 页面

- **网络总览**：核心状态、实时流量、可交互流量拓扑、吞吐和延迟 / 抖动 / 可用率。
- **设备与服务**：设备流量排行、基于目标域名的服务识别，以及通过 ping 探测候选 IP 的设备接入向导。
- **访问记录**：成功、拨号失败、无数据、拒绝和回退直连等连接结果，可按设备、出口和结果筛选。
- **设置**：外观主题、版本更新、CLI 安装、开机自启和运行日志。

App 还提供分流规则编辑器，支持拖动排序和 Clash 风格文本粘贴。智能回退学习会在默认代理拨号失败后尝试直连；同一域名在 24 小时内成功回退 3 次后，会生成一条可删除的直连规则。

## 主题

内置暖沙米、经典浅色、石墨深色和海雾蓝四套主题。主题会即时生效并由 App 自动记忆。

## 数据与隐私

- 服务名称来自网关观察到的目标域名，例如 YouTube 或 Nintendo，并不是远端设备的本地进程名。
- 连接历史只保存在内存中，最多 2000 条、保留 72 小时，不写入磁盘。
- App 通过当前出口访问 `ipwho.is` 以显示公网 IP、地区和 ISP，结果在内存缓存 30 分钟；失败不影响转发。
- fake-IP 映射会写入权限为 `0600` 的本机缓存，以便重启后恢复域名。它可能包含访问过的域名，不应公开分享。

## 安装

从 [GitHub Releases](https://github.com/Tght1211/lan-proxy-gateway/releases/latest) 下载 DMG，将 **LAN Proxy Gateway** 拖入“应用程序”后打开。DMG 内包含同版本的 `gateway` 核心，设置页也可以把 CLI 安装到 `/usr/local/bin/gateway`。

涉及网关防火墙规则、系统代理或开机自启时，macOS 会请求管理员授权。旧版核心仍在运行时，App 会提示使用当前内置核心重启。

## 本机构建

```bash
make build-app
make dmg VERSION=v0.1.0-dev
```

只有 Xcode Command Line Tools 时，`make dmg` 构建当前 Mac 架构；完整 Xcode 或 GitHub macOS Runner 会构建 universal App。
