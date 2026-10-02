# 外部 Agent Skill

应用不再内置聊天模型、API Key 设置或自动 AI 监控。用户可以把官方 Skill 安装到自己的 AI Agent，让它通过本机 gateway CLI 读取状态、诊断连接、调整手动 HTTP 代理和分流规则。

## 导出与安装

在“设置 → CLI 与 Agent”中选择“导出 Skill ZIP”，再点击“复制安装说明”，把 ZIP 和说明交给支持 SKILL.md、能够执行本机命令的 Agent。Agent 按自身支持的安装机制安装整个 `lan-proxy-gateway/` 目录，包括 `SKILL.md`、`references/` 和 `scripts/`，不需要使用网关内置 AI 工作目录。软件功能更新后，重新导出并替换 Agent 中的旧版 Skill；已安装的副本不会自行更新。

命令行也可以导出：

```bash
gateway skill export --output ./lan-proxy-gateway-skill.zip
```

默认不覆盖已有文件；需要替换时显式加 `--force`。源码构建可运行 `make skill`，得到 `dist/lan-proxy-gateway-skill.zip`。包内仅有通用操作指引、命令参考和只读检查脚本，不含本机配置、用户名密码或 API Key。

导出包可交给 Agent 的示例指令：

> 请安装或更新此 ZIP 中的 lan-proxy-gateway Skill。阅读新版说明后先检查 gateway 版本，再获取 agent snapshot 的摘要。自学习记录按状态、服务、路由和范围分类分页，暂不更改配置。

应用复制的说明还包含当前 bundle 中的 gateway 路径，防止 Agent 误用系统中另一个旧版本。Agent 需要运行在网关电脑上，或使用用户已授权的远程命令通道；代理端口不能当作管理 API 使用。

## 控制范围

- `gateway agent snapshot`：返回不含代理密码的配置与运行状态；核心未运行时仍返回配置，并明确报告 `runtime_error`。
- `gateway routing list/set`：按完整规则列表管理分流，Skill 要求保留其他规则与优先级。
- `gateway learning accept/ignore/restore/undo/configure`：管理代理学习、暂停域名及学习设置；每次命令传入一个动作和一个域名或完整设置 JSON。撤销会同时暂停学习，恢复不会立即写入规则。
- `gateway egress proxy/direct`：仅修改设备出口，不修改 Mac 系统代理和 DNS。
- `gateway hotspot status/enable/disable/use-lan`：检测与配置热点接管，保存接入变更后需重启核心，系统互联网共享保持原样。
- `gateway http-proxy set`：通过标准输入设置手动代理，保留现有认证时省略密码。
- `gateway start/restart/stop`：遵循现有系统权限，重启会中断连接。

外部 Agent 使用它自己的工具权限、模型和授权机制，Skill 不会自动获得管理员权限。导出不安装新后台服务，也不开放远程管理端口。网络元数据可能进入外部 Agent 的对话上下文，应按照该 Agent 的使用方式处理。

核心的连接统计、故障恢复和状态接口继续运行，独立“出口健康”页面与内置 AI 已移除。

## 大量自学习记录

`agent snapshot` 仅对部分连接和历史字段裁剪，规则和学习数组可能有数千条。随 Skill 附带的 `scripts/gateway_inspect.py` 使用 Python 3 标准库，在本地调用指定 CLI 并返回摘要；学习详情支持服务、路由、规则范围和搜索的组合筛选，每页默认 50 条、最多 100 条，不修改任何规则。

```text
python3 <Skill目录>/scripts/gateway_inspect.py --gateway <实际gateway路径>
python3 <Skill目录>/scripts/gateway_inspect.py --gateway <实际gateway路径> --view learning --state saved --service Google --action proxy --scope domain --page 1 --page-size 50
```

状态分为待确认、已保存、暂停学习；服务优先采用已保存的 `自动学习 · 服务名`，没有可靠识别信息时保留“未分类”。新规则为精确域名代理规则，旧版直连规则和域名后缀规则单独标明。暂停学习不等于禁止访问，不应为了缩减数量擅自把多个域名合成后缀规则。没有运行遥测时明确标记不可用，不能解释成没有记录。摘要仅供查看，不能把分页结果当作完整规则列表提交。
