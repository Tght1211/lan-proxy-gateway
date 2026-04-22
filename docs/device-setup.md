# 设备接入方式详解

装好 gateway 之后，要让其他设备用上这份代理有三种接入方式。先看 README 里的对比表决定用哪一种，这里是每一种的详细步骤 + 各系统差异。

> ⚠️ **Windows 家用版重大限制**
>
> **方式 1（改网关）在 Windows 家用版上走不通**。macOS / Linux 靠 iptables/pf 做 NAT，Windows 家用版没有 RRAS，ICS 又强制 `192.168.137.0/24` 不能自定义 —— 项目里 Windows 分支的 `ConfigureNAT` 是 no-op。手机 / iPad 请直接用方式 2（设 HTTP 代理）。
>
> Switch / PS5 / Apple TV / 智能电视这类**不能设代理只能改网关**的设备，在 Windows 上无解，请改用 Linux / macOS 主机或软路由。

---

## 📺 方式 1 · 改网关

> ⛔ **仅 macOS / Linux**。Windows 用户请跳到方式 2。

**适合**：Switch / PS5 / Apple TV / 智能电视这类**只能填网关 + DNS，不能填代理**的设备。

### 填什么

| 字段 | 填什么 |
|---|---|
| 网关 (Gateway) | 电脑的局域网 IP |
| DNS 服务器 | 同一个 IP |
| 子网掩码 | `255.255.255.0` |
| IP 地址 | 保留 DHCP 或设静态 IP |

保存 → 重连 Wi-Fi，所有流量（YouTube / 游戏 / 各类 App）自动走代理。

### 原理

设备把默认路由指向电脑 → 电脑的 iptables MASQUERADE / pf NAT 把流量转发到 mihomo TUN → mihomo 按规则分流。

各厂商具体操作步骤见 [docs/switch-setup.md](switch-setup.md) / [docs/ps5-setup.md](ps5-setup.md) / [docs/appletv-setup.md](appletv-setup.md) / [docs/tv-setup.md](tv-setup.md)。

---

## 📱 方式 2 · 填代理

**适合**：iPhone / Android / 电脑浏览器扩展 —— **能手动填代理服务器**的场景。**Windows 下所有设备只能走这条**。

### 填什么

| 字段 | 填什么 |
|---|---|
| 代理服务器 | 电脑的局域网 IP |
| 端口 | `17890`（HTTP + SOCKS5 混合端口） |
| 类型 | HTTP 或 SOCKS5 都行，端口同一个 |
| 用户名 / 密码 | 留空 |

### 具体步骤

- **Android**：Wi-Fi 设置 → 长按当前网络 → 修改网络 → 高级选项 → 代理 = 手动
- **iOS / iPad**：Wi-Fi 设置 → 点当前网络右侧 (i) → 配置代理 = 手动
- **Windows / macOS 浏览器**：装 Proxy SwitchyOmega 扩展，填 `电脑IP:17890`

完整手机配置教程（含截图）见 [docs/phone-setup.md](phone-setup.md)。

### 局限

只走 App 自己主动发到代理的流量：
- ✅ 浏览器、大部分 iOS/Android App
- ❌ Switch / PS5 等主机（根本不支持设代理）
- ❌ 某些不支持代理的原生 App（只能走方式 1 或方式 3）

---

## 💻 方式 3 · 本机也走规则

**适合**：跑 gateway 这台 PC 自己的浏览器 / CLI / IDE 也想按规则走代理。两条路：

### ① 开着 TUN（推荐，Windows 唯一可用的路）

TUN 接管本机所有出向流量，浏览器不用改任何代理设置。**装完默认就是这状态**。

> ⚠️ TUN 只劫持 TCP/UDP **数据包**，DNS 查询如果直连路由器、mihomo 看到的是 IP —— `GEOIP,CN,DIRECT` 能命中，但 `DOMAIN-SUFFIX,github.com,Proxy` 这类域名规则**命不中**。纯国内/国外分流够用；想让域名规则精确生效，看 ②。

### ② 把本机系统 DNS 改到 `127.0.0.1`

让 DNS 查询走 mihomo fake-ip，所有域名规则精确命中。

#### macOS 一键

主菜单 → `1 设备接入指引` → 按 `L`。

代码会 `networksetup -listallnetworkservices` 枚举**所有活跃网络服务**（Wi-Fi / Ethernet / Thunderbolt 等全部一起改），插着网线又开 Wi-Fi 也能覆盖到 —— 不需要自己选。恢复按 `R`。

#### Linux 手动

一键切换没实现（各发行版 systemd-resolved / NetworkManager / resolvconf 风格差异大）。命令示例：

```bash
# systemd-resolved（Ubuntu 18.04+ / Fedora）
for link in $(nmcli -t -f DEVICE con show --active | cut -d: -f1); do
    sudo resolvectl dns "$link" 127.0.0.1
done

# NetworkManager（不管有没有 systemd-resolved）
sudo nmcli con mod "Wired connection 1" ipv4.dns 127.0.0.1 && \
  sudo nmcli con up "Wired connection 1"
```

多连接场景记得每个 active 连接都改，否则系统按优先级选到没改的那个就没效。

#### Windows

一键切换没实现；也不推荐改系统 DNS（netsh 每个接口独立、改回麻烦、还要处理 IPv6）。

**推荐保持 TUN 开着**；要强制域名规则精确生效，浏览器装 SwitchyOmega 填 `127.0.0.1:17890` 即可。

#### 验证

macOS：

```bash
dscacheutil -q host -a name ping0.cc
# 返回 198.18.x.x （fake-ip）即 DNS 已走 mihomo
```

所有平台：浏览器打开 `https://ping0.cc` 查看出口 IP。

---

## 💡 方式对比

| 方式 | 覆盖面 | 需要 TUN | 适合设备 | macOS | Linux | Windows |
|---|---|---|---|---|---|---|
| 1 改网关 | 所有流量 | ✅ | Switch / PS5 / TV | ✅ | ✅ | ❌ |
| 2 填代理 | 仅支持代理的 App | ❌ | iPhone / Android / 浏览器 | ✅ | ✅ | ✅ |
| 3 本机自己 | 本机全部 | TUN 开=✅ / 关=改 DNS | 跑 gateway 这台电脑 | ✅ | ✅ | ✅（TUN 开着自动） |
