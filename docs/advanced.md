# 进阶配置

## 先记住这 4 个配置分区

项目目前所有能力都围绕这 4 个区块展开:

1. `proxy`
   代理来源: 订阅链接 / 本地配置文件 / 订阅名
2. `runtime`
   网关运行: 端口、API 密钥、TUN、局域网共享、本机绕过
3. `rules`
   规则系统: 国内直连、广告拦截、自定义规则
4. `extension`
   扩展模式: chains / script

这能保证项目持续演进时，不会把最核心的两条主线做散:

- 局域网共享
- 链式代理

---

## 链式代理: AI 账号场景的核心能力

链路示意:

```text
普通模式:  设备 -> 机场节点 -> AI 服务
chains:    设备 -> 机场节点 -> 住宅代理 -> AI 服务
```

这更适合:

- Claude / ChatGPT 注册
- Codex / Cursor 日常使用
- “普通流量继续走机场，AI 流量走住宅出口”的场景

### 配置方法

```bash
gateway chains
sudo gateway restart
```

向导会询问:

1. 住宅代理地址和端口
2. 用户名和密码
3. 机场测速组名
4. `rule / global` 路由模式

### 模式差异

- `rule`
  普通流量继续走机场，AI 相关流量走住宅出口
- `global`
  所有流量走住宅出口

### 查看结果

```bash
gateway status
```

状态页会显示:

- 入口节点
- 普通出口
- 住宅出口

这样用户可以直观看到是否真的形成了链式代理。

---

## 扩展脚本

如果你已经有 Clash Verge Rev 的脚本，或者要做更复杂的自定义逻辑，可以使用 `script` 模式。

```yaml
extension:
  mode: script
  script_path: /etc/gateway/my-script.js
```

也可以直接切换:

```bash
gateway switch extension script /etc/gateway/my-script.js
sudo gateway restart
```

项目内自带示例:

```text
./script-demo.js
```

---

## 规则系统

规则不再依赖模板硬编码，而是有独立的构建逻辑，默认围绕中国用户的真实使用场景准备了:

- 局域网 / 私网地址直连
- 微信、QQ、腾讯生态直连
- 小红书、抖音、头条等国内常见平台直连
- 王者荣耀等国内常见游戏与平台直连
- Apple 常见服务分流
- Nintendo 服务代理
- 常见广告 / 跟踪域名拦截
- 国外常见网站和 AI 服务代理

### 自定义规则

```yaml
rules:
  extra_direct_rules:
    - "DOMAIN-SUFFIX,corp.example.com,DIRECT"
  extra_proxy_rules:
    - "DOMAIN-SUFFIX,example-overseas.com,Proxy"
  extra_reject_rules:
    - "DOMAIN-SUFFIX,annoying-ads.example,REJECT"
```

### 交互式编辑

```bash
gateway config
```

进入:

- `规则开关与自定义规则`

就能不用手写 YAML 来编辑。

---

## 本机绕过代理

很多用户的真实需求不是“让这台电脑也一起翻”，而是:

- 这台电脑继续保持本地网络习惯
- 局域网里的 Switch / PS5 / 手机 / 电视 通过这台电脑共享出去

现在可以通过下面的开关实现:

```yaml
runtime:
  tun:
    enabled: true
    bypass_local: true
```

建议通过配置中心设置:

```bash
gateway config
```

然后进入:

- `局域网共享 / TUN / 端口`

---

## 普通权限控制

启动、停止、重启等系统级动作通常需要 root / 管理员权限。为了降低 AI 客户端和普通用户的操作负担，现在支持:

```bash
gateway permission print
sudo gateway permission install
gateway permission status
```

配置后，CLI 会自动尝试 `sudo -n` 提权，而不是每次都要求用户手动把整条命令改成 `sudo ...`。

---

## 运行中 TUI 控制台

执行 `gateway start --tui`，或者网关已经运行后执行 `gateway console --tui`，会进入运行中控制台。默认的 `gateway start` 现在会先进入兼容性更好的简单模式。

你可以在里面:

- 用 `/status` 看完整状态
- 用 `/config` 进入配置中心
- 用 `/chains` 看扩展状态
- 用 `/logs` 看日志
- 用 `/update` 看升级提示
- 用 `Ctrl+P` 选择策略组和节点

这也是这个项目下一阶段重点打磨的交互核心。

---

## AI Skill

查看 skill 信息:

```bash
gateway skill
gateway skill path
```

仓库里已经带了一个场景化 skill，目标是让 AI 客户端以“任务流”的方式操作这个系统，而不是让用户自己记住所有命令和配置细节。

推荐覆盖场景:

- 新机器开通
- 局域网共享排障
- chains 配置与验证
- 节点切换
- 本机绕过开关
- 健康检查和日志排查

---

## 自动更新

系统现在支持两种升级体验:

1. 主动升级

```bash
sudo gateway update
```

2. 被动提醒

- `gateway`
- `gateway start`
- 运行中 TUI 控制台

都会在检测到新版本时提示升级，但不会每次都强制联网阻塞，内部做了缓存。

---

## 服务化与稳定性

安装系统服务后，机器重启后网关自动拉起:

```bash
sudo gateway service install
```

适合:

- 家里长期在线的 Mac mini / Linux 小主机
- 做成“轻软路由替代方案”
- 给全屋设备持续提供透明代理能力
