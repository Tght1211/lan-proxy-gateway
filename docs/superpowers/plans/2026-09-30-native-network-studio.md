# Native Network Studio Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将已确认的 HTML 交互落地为唯一原生 LAN Proxy Gateway 界面。
**Architecture:** 复用 SwiftUI、AppModel、GatewayClient 与现有 Go 接口。纯数据投影和绘图解耦，每个功能页独立文件，节点配置统一 sheet 路由。
**Tech Stack:** SwiftUI / Charts / Foundation / Security / Go.
**Spec:** docs/design/native-network-studio.md

## Global Constraints
- 名称 LAN Proxy Gateway；保留所有现有未提交改动。
- 只展示真实核心能力和数据；多源核心范围待用户答复。
- Wi-Fi 密码使用 Keychain；演示数据不进入产品。
- 不自动重启网关或改动系统网络配置。

## Review Focus
- 核心离线、无设备时不伪造路由或健康。
- 连接重复采样不会重复无限播放；重启后 ID 复用可恢复。
- HTTP/PAC 无法识别时共用入口，不猜测方式。
- 域名颜色稳定，IPv4 热点归属使用真实 CIDR。
- 保存错误、取消编辑保留原配置。

## Tasks
- [x] 1. 基线：Go 全量测试、Swift 构建；记录既有失败。保存起始 diff。
- [x] 2. 外壳：AppSection 六模块、唯一品牌、取消模式入口；主题模型迁出 ContentView。
- [x] 3. NetworkPresentation.swift：连接投影／播放缓存／入口归类，模型测试覆盖去重、空态、重启、未知入口。
- [x] 4. NetworkTopologyView.swift、NetworkOverviewView.swift、NetworkSheets.swift：横向裸图标、真实事件播放、浮字、模态编辑，稳定性和一分钟吞吐独立卡片。
- [x] 5. NetworkExitsView.swift、NetworkRulesView.swift：出口卡片及三个规则 tab，复用真实编辑器／学习设置，空态可配置。
- [x] 6. NetworkDevicesView.swift、WiFiGuideView.swift、WiFiMemoStore.swift：接入 tabs、短指南、设备列表与 Keychain 备忘。
- [x] 7. SettingsView：五个分类、已有控制能力、版本和 CLI/Skill、日志过滤／暂停及文件位置。
- [x] 8. 编译、模型和 Go 测试；独立审查；生成本地 App 检查布局，记录不支持的核心能力。

## Ledger
- 使用当前工作目录继续，因为最新热点／自动学习修改尚未提交；不创建基于旧 HEAD 的分叉，避免遗漏用户改动。
- 用户已明确批准演示并要求开始开发；直接执行，不再设置额外设计审批。

- Ruling: 本轮按现有单代理核心交付原生六模块；用户未回答多源核心范围问题。多源池、独立 C、规则兜底链后续扩展；成本是这些演示能力暂未进入实际核心。
- Ruling: 保留当前热点／静态网关互斥限制并在设置中说明；HTTP/PAC可同时启用。成本是尚未满足两种透明接入同时接管。
- Ruling: 版本页提供真实探测和官方发布页下载；尚无 App 原位升级实现，不能把链接标为自动升级。成本是升级仍需安装发布包。
- 验证：Go 全量测试、app/relay race 测试、Swift SDK15.2 构建、Foundation 模型和拓扑回放测试通过。默认SDK27缺宏插件为已记录环境问题。
- 新增首响应数据耗时；等待实际响应/终止结果后有界回放，不伪造响应速度。
- 代码审查：三项 Important（热点开关、规则草稿外部更新、HTTP代理等待参数）；逐项修复。规则修订判断及HTTP独立预算回归先失败后通过。
- 实机QA待完成：Mac锁屏，CUA自动解锁失败；已请用户解锁。Keychain写入和实体热点操作未执行，不更改现有网关运行态。

- Final: fixed HTTP独立代理拨号预算 — TestExplicitFallbackUsesProxyBudget RED→GREEN，全量Go测试及race通过。
- Final: fixed 未保存草稿覆盖外部新增规则 — NetworkRuleRevision clean-reload/dirty-conflict/UUID兼容 RED→GREEN，模型测试及构建通过。
- Final: fixed 热点开关读取错误字段 — 改为hotspot.enabled，编译通过；锁屏阻止点击实机验证。
- Final: Ruling: 未执行真实Keychain写入或物理热点变更 — 保留现有密码与网络，等待用户实机检查；成本是这些系统操作尚未端到端验证。
- Final: Ruling: 不能以代码审查代替窗口布局与可访问性检查 — Mac锁屏，任务8仍未完成；成本是布局问题可能留待解锁后修正。
- 预览包：dist/LAN Proxy Gateway Native Preview.app，代码签名验证通过，包含新核心但现有后台服务没有替换。

## 本轮还原与设备修正
- [x] HTML最终配色、公共控件尺寸、裸线性图标、侧栏选中态。
- [x] 总览四项指标、紧凑横向拓扑、独立等高网络稳定性/一分钟吞吐。
- [x] 出口双列卡片、自学习三步与三列参数、规则/设备/设置Tab、Wi-Fi备忘字段。
- [x] 实际字节活动时间；10分钟灰色/24小时隐藏，新数据恢复；新旧核心兼容。
- [x] 基于近期多个平台线索识别手机/电脑/主机，避免访问游戏商店即识别主机。
- [x] Go全量测试和Foundation模型回归通过（新增回归先失败后修复）。
- [x] 解锁后完成原生六页面视觉检查；拓扑网关、策略、规则、代理、直连、拒绝、互联网、自学习弹窗实际点击检查。

## 解锁后实机验收（2026-09-30）
- 修正拓扑连接线穿过图标、三个设备间距不均；静默设备灰色但仍可查看详情，新流量恢复。
- 下拉框改为统一的带边框按钮和选项弹层；修正 Tab 无障碍标签及输入框双重边框。
- 规则编辑弹窗加宽至 940，验证交互式/文本式切换；自学习记录去掉旧嵌套标题和展开控件。
- 设备接入摘要去掉多余复制按钮，设备副标题展示接入方式；日志端口不再添加千位分隔符。
- 检查实时日志展示及日志位置；重新构建、Foundation 模型回归、签名校验通过。
- 本次只重启 UI；后台核心保持现状。旧核心不支持的学习等待参数在界面中禁用并说明。
- 系统热点变更、Keychain写入、实际升级安装没有执行；多代理池、并行透明接入与原位升级仍属前述未完成能力。
