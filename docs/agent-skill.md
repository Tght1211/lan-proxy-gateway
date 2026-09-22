# 外部 Agent Skill

应用不再内置聊天模型、API Key 设置或自动 AI 监控。用户可以把官方 Skill 安装到自己的 AI Agent，让它通过本机 gateway CLI 读取状态、诊断连接、调整手动 HTTP 代理和分流规则。

## 导出与安装

在“设置 → 外部 Agent Skill”中选择“导出 Skill ZIP”，再点击“复制安装说明”，把 ZIP 和说明交给支持 SKILL.md、能够执行本机命令的 Agent。Agent 按自身支持的安装机制安装整个 `lan-proxy-gateway/` 目录，不需要使用网关内置 AI 工作目录。

命令行也可以导出：

```bash
gateway skill export --output ./lan-proxy-gateway-skill.zip
```

默认不覆盖已有文件；需要替换时显式加 `--force`。源码构建可运行 `make skill`，得到 `dist/lan-proxy-gateway-skill.zip`。包内仅有通用操作指引和命令参考，不含本机配置、用户名密码或 API Key。

导出包可交给 Agent 的示例指令：

> 请安装此 ZIP 中的 lan-proxy-gateway Skill。安装后先检查 gateway 版本，运行 agent snapshot，说明当前代理和运行状态，暂不更改配置。

应用复制的说明还包含当前 bundle 中的 gateway 路径，防止 Agent 误用系统中另一个旧版本。Agent 需要运行在网关电脑上，或使用用户已授权的远程命令通道；代理端口不能当作管理 API 使用。

## 控制范围

- `gateway agent snapshot`：返回不含代理密码的配置与运行状态；核心未运行时仍返回配置，并明确报告 `runtime_error`。
- `gateway routing list/set`：按完整规则列表管理分流，Skill 要求保留其他规则与优先级。
- `gateway http-proxy set`：通过标准输入设置手动代理，保留现有认证时省略密码。
- `gateway start/restart/stop`：遵循现有系统权限，重启会中断连接。

外部 Agent 使用它自己的工具权限、模型和授权机制，Skill 不会自动获得管理员权限。导出不安装新后台服务，也不开放远程管理端口。网络元数据可能进入外部 Agent 的对话上下文，应按照该 Agent 的使用方式处理。

核心的连接统计、故障恢复和状态接口继续运行，独立“出口健康”页面与内置 AI 已移除。
