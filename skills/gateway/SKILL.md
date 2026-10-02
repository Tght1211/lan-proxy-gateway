# LAN Proxy Gateway · 外部 Agent

可安装的官方 Skill 现在随 gateway 核心发布，可通过应用设置导出 ZIP，或运行 `gateway skill export --output lan-proxy-gateway-skill.zip`。

- [安装与使用](../../docs/agent-skill.md)
- [维护中的 Skill 源码](../../internal/agentskill/content/lan-proxy-gateway/SKILL.md)

完整 Skill 安装请使用导出的 ZIP 并保留 `references/`、`scripts/`；软件更新后重新导出并更新 Agent 中的副本。新版包含自学习分类、分页摘要、出口与热点接入控制，设置入口为“CLI 与 Agent”。本目录仅为源码中的参考入口，不是另一套可安装 Skill。
