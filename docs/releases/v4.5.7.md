# v4.5.7 — 图文使用手册与 Linux CLI 验证工具

发布于 2026-11-06。

本次更新重点是新手文档、安装包选择说明和可复用的 Linux 测试工具。App 与代理核心延续 v4.5.6 的功能代码；不是 Linux UDP 修复版本。

## 本次更新

- **从安装到接通设备的图文手册**：新增 16 张真实界面截图，按安装、上游代理、设备接入、访问验证、更新及停止恢复的顺序说明。覆盖 Wi-Fi 热点、HTTP / PAC、静态网关、规则和自动学习。
- **重写中英文 README**：把首次使用收敛为四步，区分网关电脑安装包和客户端设备设置，补齐常见问题与各类设备的接入说明。
- **Release 安装包对照表**：正文顶部按 Mac 图形 App、Mac / Linux 命令行及 CPU 架构列出下载文件。Mac 用户选择 universal DMG，手机和游戏机不用安装本项目。
- **Docker CLI 验证工具**：提供可独立运行的冒烟测试、UDP 模式切换测试和双容器透明转发测试。自动创建并清理临时网络与容器；失败返回非零退出码。
- **公开测试结果与限制**：归档 Linux ARM64 实测、Desktop 功能对照和缺陷复现记录，明确区分已有实现与验证通过。
- **同步源码与已发布版本**：主分支合入 v4.5.3–v4.5.6 已发布代码，保留 App 内更新、来源 IP 免密、规则差异合并、并发写入保护及上游协议切换修复，避免从旧主分支重新构建导致功能回退。

[完整图文手册](https://github.com/Tght1211/lan-proxy-gateway/blob/v4.5.7/docs/user-guide.md) · [安装包与命令行安装说明](https://github.com/Tght1211/lan-proxy-gateway/blob/v4.5.7/docs/download.md) · [Linux 测试报告](https://github.com/Tght1211/lan-proxy-gateway/blob/v4.5.7/docs/testing/linux-audit-2026-11-06.md) · [Docker 测试工具](https://github.com/Tght1211/lan-proxy-gateway/tree/v4.5.7/tools/linux-audit)

## 升级说明

- 已使用 v4.5.6 的用户，运行功能保持一致；可以直接阅读新手册和测试报告，不需要为文档更新重新配置网络。
- macOS 更新 App 后，按提示更新并重启后台核心；CLI 更新对应平台的程序，并重启运行中的核心或系统服务。重启会短暂中断现有连接。
- 保留已有上游代理、认证、规则、学习记录和接入配置。本次不自动开启热点、不修改设备网络参数。
- 本 Release 提供 macOS universal DMG、macOS / Linux 的 arm64 / amd64 CLI 及压缩包、Agent Skill ZIP 和 SHA256SUMS。

## 已知限制：Linux 用户请先阅读

- **Linux fake-IP UDP 路径存在故障，尚未修复。** 容器接入测试中，REDIRECT 后无法正确恢复原始 fake-IP 目标，UDP 数据被丢弃；不能把本版本视为游戏语音等 UDP 场景已通过验证。这个结论不等于所有普通直连 UDP 都失效。
- **从直连运行状态切到代理，UDP 监听器不会自动创建。** 重启核心后才出现监听器；重启不能解决上面 Linux 原始目标恢复的问题。这段生命周期逻辑由各平台共用。
- **Linux 全量 Go 测试仍有编译障碍。** 公共防火墙测试引用了 macOS 专属类型；主程序可编译，但完整 Linux 测试套件尚未通过。
- systemd 开机自启、安装升级回滚、Linux amd64 运行，以及真实局域网设备兼容性未纳入本轮容器验收。Mac 热点引导与系统代理自动配置没有对应的 Linux 实现。

## 验证范围

- 合并后的 App、CLI 与核心源码与 v4.5.6 一致；运行代码不新增变更，补充文档、发布流程和测试工具。
- 本地 Go 全量 race 测试与 Swift 模型 / 更新器 / 规则编辑回归。
- Linux ARM64 新版本构建在 Docker 中通过 28 项 CLI 冒烟检查，包括真实 iptables 应用、HTTP / CONNECT / PAC、认证、HTTP/SOCKS5 上游、规则热应用、DNS、重启及停止清理。
- v4.5.6 双容器接入测试中 DNS 和 TCP 透明转发通过，UDP 失败；已知失败保留在测试报告中，不计为通过。
- 发布工作流再次运行 macOS Go / Swift 测试，构建各平台附件与 universal DMG，并生成校验清单。交叉编译不等于 Linux amd64 运行验收。
