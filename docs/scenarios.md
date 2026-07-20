# 典型场景（v4）

## 场景一：本机跑着 clash / sing-box，让整屋设备共享这份代理

1. clash / sing-box 照常运行（假设 SOCKS5 端口 `127.0.0.1:7897`）
2. `gateway system-proxy on --type socks5 --host 127.0.0.1 --port 7897`
3. `sudo gateway start`
4. 设备（Switch / PS5 / Apple TV / 电视 / 手机）把 **网关 + DNS** 改成这台电脑的局域网 IP

完成。设备的 TCP 流量被透明转发到 gateway，再由它通过 7897 发给 clash。
节点选择和复杂分流规则均在 clash / sing-box 中管理。

---

## 场景二：只要一台"笨"旁路由（不翻墙）

`gateway system-proxy off` + 启动。设备改网关+DNS后流量经 gateway 直出，
等于一台纯转发路由器。适合：

- 临时给不能配代理的设备提供网络
- 代理软件挂掉时的保底通道（一条命令切回直连，全屋不断网）

---

## 场景三：上游代理在另一台机器上

上游不必须在同一台机器。比如 NAS 上跑着 sing-box（`192.168.1.5:1080`）：

```bash
gateway system-proxy on --type socks5 --host 192.168.1.5 --port 1080
```

注意：如果那台 NAS 自己也把网关指向本机，它的代理流量会作为普通转发流量
经过本机——能工作，只是多一跳。

---

## 场景四：白天直连、晚上走代理

```bash
gateway system-proxy on --type socks5 --host 127.0.0.1 --port 7897  # 切代理
gateway system-proxy off                                            # 切直连
```

热切换只增删一条防火墙规则：**已建立的连接不断，新连接走新路径**。
可以安心写进 crontab。

---

## 验证清单（排错用）

在**设备**上：

```bash
dig example.com          # 默认返回真实 IP
curl ifconfig.me         # 代理模式显示上游出口 IP；直连模式显示家里宽带 IP
curl --http3 -I https://www.google.com   # 代理模式应快速失败并回退 TCP
```

在 **gateway 主机**上：

```bash
gateway status                      # running / egress / ports
tail -f ~/.config/lan-proxy-gateway/gateway.log
# Linux:
sudo iptables-save | grep lan-proxy-gateway
# macOS:
sudo pfctl -s nat -a com.apple/lan-proxy-gateway
```
