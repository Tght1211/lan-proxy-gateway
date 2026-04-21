# LAN Proxy Gateway v3.1.1

紧跟 v3.1.0 的小补丁，围绕两个主人实测踩到的坑。

## 1. 本机浏览器访问 ping0.cc 没走规则 —— 方式 3「本机自己用」

**现象**：订阅配好、Web UI 里 AI 落地节点组也正常，但本机浏览器打开 `ping0.cc` 看到的出口 IP **不是**住宅 IP，而是订阅机场节点。

**根因**：macOS 系统 DNS 默认指向 LAN 路由器（如 `192.168.12.1`），不走 mihomo。本机程序查 DNS 直接拿到真 IP，mihomo 只能按 GeoIP 匹配，**所有 `DomainSuffix` 类规则对本机失效**。

**新功能**：
- 「设备接入指引」页面加入**方式 3 · 本机自己用**说明
- macOS 支持**一键切**：主菜单 → `1 设备接入指引` → `L` 把本机 DNS 切到 `127.0.0.1`；`R` 恢复
- 底层走 `networksetup -setdnsservers` + `dscacheutil -flushcache`
- 状态灯 ●/○ 显示当前本机 DNS 是否指向 `127.0.0.1`
- Linux / Windows 暂不做一键（systemd-resolved / NetworkManager / netsh 各自有坑），guide 里打印手动命令

**验证**：切换后 `dscacheutil -q host -a name ping0.cc` 应返回 `198.18.x.x`（mihomo 的 fake-ip），然后浏览器访问 ping0.cc 会命中 `DOMAIN-SUFFIX,ping0.cc,🛬 AI落地节点`。

## 2. 菜单对齐修复 —— CJK / Emoji 宽度

v3.1.0 的切换节点、换代理源菜单里遇到中文和 emoji 就对不齐：

```
  1  Proxy                     当前: cc
  2  🛫 AI起飞节点                  当前: cc              ← 太靠右
  3  🛬 AI落地节点                  当前: 🏠 住宅IP        ← emoji 对不齐
```

根因：`fmt.Printf("%-20s", ...)` 按**字节数**填空格，中文一字 3 字节但显示占 2 列，emoji 一字 4 字节但显示 2 列，全错位。

加了 `displayWidth` + `padRightWide` 按**显示列数**对齐，CJK / Emoji / 全角都算 2 列。三处表格（代理源 / 切节点顶层 / 分组内节点）全部换掉。

## 3. 设备接入指引 · 紧凑改版

v3.1.0 的指引是长段落叙述，主人反馈"看着眼花"。v3.1.1 压成一屏：

```
  本机 IP 192.168.12.100  ·  路由器 192.168.12.1  ·  代理端口 17890 (HTTP+SOCKS5)

  📺 方式 1 · 改网关      Switch / PS5 / Apple TV / 智能电视
     网关 → 192.168.12.100    DNS → 192.168.12.100    掩码 → 255.255.255.0

  📱 方式 2 · 填代理      iPhone / 电脑 App / 浏览器插件
     代理服务器 → 192.168.12.100    端口 → 17890

  💻 方式 3 · 本机自己用  跑 gateway 这台电脑的浏览器 / App 也想走规则
     把本机 DNS 改到 127.0.0.1，让本机查询走 mihomo
     macOS：sudo networksetup -setdnsservers Wi-Fi 127.0.0.1

  💡 方式 1 需 TUN+DNS 都开；方式 2/3 只要 gateway 在跑
  💡 验证方式 3：dscacheutil -q host -a name ping0.cc 返回 198.18.x.x 说明生效
```

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.1.1/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.1.1/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.1.1/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.1.1/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.1.1/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.1.1/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.1.1/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.1.1/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.1.1/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.1.1/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.1.1/SHA256SUMS)
