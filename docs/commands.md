# 命令行参考

[文档目录](README.md) · [CLI 安装](download.md#linux) · [图文手册](user-guide.md)

Mac App 用户无需执行这些命令。示例端口和地址应替换为自己的配置；下面需要管理员权限的命令会修改服务或网络状态。

## 查看状态

```bash
gateway --version
gateway status
gateway status --json
gateway service status
gateway hotspot status
```

状态包含配置路径；日志路径以实际状态为准。`gateway status` 给出的候选设备 IP 不保证未占用，子网掩码应按实际网络填写。`hotspot status` 是 macOS 热点检测，不会替你开启系统互联网共享。

## 初始化与核心启停

```bash
sudo gateway install             # 首次配置、启动、选择是否开机自启
sudo gateway start
sudo gateway start --foreground  # 前台排障
sudo gateway restart             # 会中断当前连接
sudo gateway stop
```

App 和 CLI 连接同一个后台核心。退出界面或终端菜单不等于停止核心；停用前先为设备 [恢复网络](user-guide.md#stop)。`sudo gateway` 不带参数会进入简易控制面板。

## 只配置设备流量出口

```bash
gateway egress proxy --type http --host 127.0.0.1 --port 7897
gateway egress proxy --type socks5 --host 127.0.0.1 --port 7897
gateway egress direct
```

选择与上游对应的一种协议即可。这些命令保留 Mac 的系统代理和 DNS；上游软件必须已经运行。配置保存后需要通过实际新连接验证。

## macOS 热点模式

先在系统完成以太网到 Wi-Fi 的互联网共享，再操作接管：

```bash
gateway hotspot enable
sudo gateway restart
```

停止接管或切回静态网关分别使用下列命令；按目的选择，不要整段依次执行：

```bash
gateway hotspot disable  # 停止接管，保留系统热点
# 或 gateway hotspot use-lan  # 切换到静态网关模式
sudo gateway restart
```

CLI 的模式修改保存配置后要重启应用；App 中的对应操作会协调重新应用与确认。系统热点名称、密码、信道仍在 macOS 设置。

## HTTP / PAC

建议使用 App 的设置界面。CLI 配置通过 `gateway http-proxy set` 的标准输入传入 JSON。结构、保留密码和免密来源字段见 [Agent 命令参考](../internal/agentskill/content/lan-proxy-gateway/references/commands.md#lan-manual-http-proxy)，使用与运行核心一致的版本。

保存字段不是任意局部补丁：先读取现有状态，保留未打算修改的端口、认证、用户名等配置；不要将凭据放在 shell 历史中。PAC 共用该 HTTP 服务，不需要另开端口。

## 分流规则

```bash
gateway routing list --json
```

写入使用 `gateway routing set --rules-json '<完整规则数组>'`。**这是替换整份规则，不是追加一条。** 先读取、保留其他规则与原顺序，再构造完整数组；不要把单条示例当成可直接执行的追加命令。新手用 App 的“配置规则 → 添加 → 查看变更 → 应用规则”更容易检查影响。

类型为 `src-ip`、`domain`、`domain-suffix`、`ip-cidr`；动作是 `proxy`、`direct`、`reject`。设备策略优先，同类规则按全局顺序首次命中。

## 系统服务

```bash
sudo gateway service install
sudo gateway service uninstall
gateway service status
```

macOS 使用 launchd，Linux 使用 systemd。卸载网关前应先移除开机服务、停止核心并恢复客户端网络，不能只删除可执行文件。

## 更新

```bash
gateway update
```

下载阶段保留当前用户代理环境，需要替换程序时再提权。正在运行的核心更新后会尝试恢复运行；检查命令回执、`gateway --version`、`gateway status` 和设备新连接。失败时先读错误，不要反复重启或删配置。

`--yes` 会跳过确认，留给明确授权的自动化场景。更新器可能显示 v3→v4 的历史迁移提醒；v3 用户按 [迁移说明](migration-v4.md) 处理，普通 v4 小版本升级不需要先删除现有配置。App 用户优先用 [App 更新流程](user-guide.md#update)。

## macOS 系统代理（独立的高级功能）

```bash
gateway system-proxy status --json
gateway system-proxy on --type http --host 127.0.0.1 --port 7897
gateway system-proxy off
```

这些命令明确修改 macOS 网络服务的系统代理，并同步网关出口；`off` 会切回直连。仅想给局域网设备共享代理时用 `gateway egress`，不要把系统代理开关当作必做步骤。
