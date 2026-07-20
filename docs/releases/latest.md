# lan-proxy-gateway v4.0.4

这是 v4 的正式稳定版本，面向常驻 Mac mini 和低功耗 Linux 小主机。Switch、PS5、Apple TV、iPhone、Android 和电视无需安装代理 App，只需配置静态 IP、网关和 DNS，即可共享宿主机现有的 Clash、Mihomo、sing-box 或其他 HTTP/SOCKS5 代理。

## 小白接入体验

- `gateway status` 和终端菜单现在直接按设备页面逐项打印配置，不再只给一个抽象的“本机 IP”。
- 自动给出同网段设备 IP 建议值，默认优先使用 `.112`，并避开 gateway 电脑和路由器地址。
- 输出完整包含：手动/静态 IP、设备 IP、子网掩码、Android 前缀长度、网关、DNS 设置、首选/备用 DNS、设备代理和 Android 私人 DNS。
- 明确只有“设备 IP”是手机或游戏机自己的地址且每台不同；网关和两个 DNS 都填 gateway 电脑 IP。
- 推荐 IP 只是建议值，使用前仍需确认未占用，建议在路由器中固定。

## README 与设备教程

- README 重新按“准备、安装、设备配置、代理出口、验证”组织，新用户可以从顶部直接完成接入。
- 新增带标注的 Nintendo Switch 与 Android 静态网络设置实图。
- 加入可直接复制给 Codex、Claude Code 等终端 AI 的 Prompt，让 AI 自动检查、安装、启动并输出填写清单。
- 新增 Clash Verge 同机示例：地址通常为 `127.0.0.1`，端口示例为 `7897`，实际值必须以代理软件界面为准。
- 统一 Switch、PS5、手机、电视、Apple TV 和 FAQ 的 DNS 说明，删除旧文档中的冲突配置。
- 修复 README 中 Star History 动态图片报错导致的破图，改用 GitHub Stargazer 数据生成仓库内曲线图，并保留在线入口。

## 稳定性说明

- 网络转发核心沿用已完成真实局域网验证的 v4.0.3：HTTP CONNECT 隧道、长连接和 Linux 零拷贝路径保持不变。
- 代理模式继续拒绝 QUIC（UDP/443），让浏览器和 YouTube 回退到可代理的 TCP；gateway 与第三方代理均无需开启 TUN。
- gateway 不提供节点、订阅或流媒体解锁，最终速度和可用性由第三方代理软件的节点及分流规则决定。

## 升级说明

### 从任意 v4.0.x 升级

可直接执行：

```bash
gateway update v4.0.4
```

现有 v4 配置可以继续使用。升级后建议执行 `sudo gateway restart`，再运行 `gateway status` 查看新的设备填写清单。

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
