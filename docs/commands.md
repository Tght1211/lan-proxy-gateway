# 完整命令说明

## 常用起步命令

| 命令 | 说明 | 需要管理员权限 |
|---|---|:---:|
| `gateway install` | 初始化向导: 下载 mihomo、录入订阅、生成配置文件 | 否 |
| `gateway config` | 交互式配置中心: 代理来源 / 局域网共享 / 规则 / 扩展 | 否 |
| `gateway config show` | 查看当前配置摘要 | 否 |
| `sudo gateway start` | 启动网关，并在交互终端里进入运行中控制台 | 是 |
| `sudo gateway start --simple` | 启动网关，并进入纯命令控制台 | 是 |
| `sudo gateway console` | 不重启网关，重新进入运行中控制台 | 是 |
| `sudo gateway console --simple` | 不重启网关，重新进入纯命令控制台 | 是 |
| `sudo gateway stop` | 停止网关 | 是 |
| `sudo gateway restart` | 重启网关 | 是 |
| `gateway status` | 查看运行状态、入口节点、普通出口、住宅出口 | 否 |

## 运行中控制台

`gateway start` 在交互终端中成功启动后，会进入一个运行中 TUI 控制台。退出后，也可以用 `gateway console` 再次进入。

支持:

- slash 命令: `/status` `/summary` `/config` `/config open` `/chains` `/nodes` `/logs` `/help`
- 顶部 tab: `Esc` 回顶部，`←/→` 切分区，`↓ / Enter` 回到功能列表
- 节点选择器: `Ctrl+P`
- 刷新反馈: `R` 会刷新当前页面，并给一个很短的脉冲反馈
- 确认交互: `/stop` `/restart` 后输入 `y / n`
- 纯命令模式: `sudo gateway start --simple` 或 `sudo gateway console --simple`

这让它更像一个持续运行的 CLI 系统，而不是“一次性打印信息就退出”的命令。

## 配置与切换

| 命令 | 说明 |
|---|---|
| `gateway switch` | 查看当前代理来源和扩展模式 |
| `gateway switch url` | 切换到订阅链接模式 |
| `gateway switch file /path/to/config.yaml` | 切换到本地 Clash / mihomo 配置文件模式 |
| `gateway switch extension` | 查看当前扩展模式 |
| `gateway switch extension chains` | 启用内置链式代理 |
| `gateway switch extension script /path/to/script.js` | 启用 JS 扩展脚本 |
| `gateway switch extension off` | 关闭扩展模式 |
| `gateway chains` | 打开链式代理向导 |
| `gateway chains status` | 查看链式代理当前配置 |
| `gateway chains disable` | 关闭链式代理模式 |

## 局域网共享 / TUN / 本机绕过

| 命令 | 说明 |
|---|---|
| `gateway tun on` | 开启 TUN 透明代理模式 |
| `gateway tun off` | 关闭 TUN 模式 |

现在更推荐直接通过 `gateway config` 来统一调整:

- `runtime.tun.enabled`
- `runtime.tun.bypass_local`
- 运行端口
- API 密钥

其中 `bypass_local` 的用途是:

- 当前这台电脑自己尽量不走科学上网
- 局域网其他设备继续通过它共享网关能力

## 策略组与节点切换

有两种方式:

1. Web 面板: `http://你的局域网IP:9090/ui`
2. CLI 运行中控制台:
   - 启动网关
   - 按 `Ctrl+P`
   - 选择节点分组
   - 选择节点并回车切换

这让它更接近一个 CLI 版的 Clash Verge Rev 工作台。

## 健康检查与维护

| 命令 | 说明 | 需要管理员权限 |
|---|---|:---:|
| `sudo gateway health` | 健康检查；异常时尝试修复 | 是 |
| `sudo gateway update` | 升级到最新版本，自动尝试镜像下载 | 是 |
| `gateway permission print` | 打印 sudoers 配置片段 | 否 |
| `sudo gateway permission install` | 安装免密控制规则，之后可普通权限触发自动提权 | 是 |
| `gateway permission status` | 查看权限控制状态 | 否 |

## 服务管理

| 命令 | 说明 | 需要管理员权限 |
|---|---|:---:|
| `sudo gateway service install` | 安装开机自启服务 | 是 |
| `sudo gateway service uninstall` | 卸载开机自启服务 | 是 |

## AI Skill

| 命令 | 说明 |
|---|---|
| `gateway skill` | 查看可供 AI 客户端安装的 skill 信息 |
| `gateway skill path` | 输出 skill 目录路径 |

skill 的目标是让 AI 客户端能直接按场景调用这个系统，例如:

- 开通局域网共享
- 配置 chains
- 切换策略组
- 打开本机绕过
- 做健康检查和日志排障

## 全局参数

| 参数 | 说明 | 默认值 |
|---|---|---|
| `--config <路径>` | 指定配置文件路径 | `./gateway.yaml` |
| `--data-dir <路径>` | 指定运行数据目录 | `./data` |

示例:

```bash
sudo gateway start --config /etc/gateway/gateway.yaml --data-dir /var/lib/gateway
```
