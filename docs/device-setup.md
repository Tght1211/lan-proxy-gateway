# 设备接入：先选一种方式

[返回图文手册](user-guide.md)

软件安装在 Mac / Linux 主机上，手机、电视、游戏机只配置网络。先按条件选一行，完成后验证，不要混用不同行的参数。

| 方式 | 主机条件 | 设备 IP / 网关 / DNS | 设备代理设置 | 详细步骤 |
|---|---|---|---|---|
| Wi-Fi 热点（推荐） | Mac 有线联网、系统互联网共享已开、App 接管已开 | 自动获取 | 关闭 | [热点主线](user-guide.md#wifi) |
| HTTP 代理 | 主机与设备可互访，HTTP 监听已保存启用 | 保持原有设置 | 填主机局域网 IP、已保存端口与凭据 | [HTTP](user-guide.md#http) |
| PAC 自动代理 | 同上，客户端支持自动代理 | 保持原有设置 | 填 App 显示的 PAC URL | [PAC](user-guide.md#pac) |
| 静态网关（进阶） | Mac / Linux 与设备在同一子网，已启用局域网接管 | 设备设空闲静态 IP；网关指向主机；按配置使用网关 DNS | 关闭 | [静态接入](user-guide.md#static) |

热点接管和静态网关共用一个透明接入模式，切换会影响当前设备；HTTP/PAC 可以同时开启。HTTP/PAC 只覆盖遵循代理设置的应用。

## 静态地址不要照抄

`gateway status` 的 IP 是建议值，不保证未占用。子网掩码也要使用真实网络值；`255.255.255.0` / `24` 只是一种常见情况。主机地址与设备地址不能相同。

默认静态接入不劫持其他 DNS；需要网关还原域名时，先确认其 DNS 已启用，再把设备 DNS 指向主机。详见 [静态接入中的 DNS 说明](user-guide.md#static)。

## 设备菜单

[手机 / 平板](phone-setup.md) · [Switch](switch-setup.md) · [PS5](ps5-setup.md) · [Apple TV](appletv-setup.md) · [智能电视](tv-setup.md)

## 验证和恢复

接入后，先打开实际网页或商店，再在“流量记录”查看设备新连接及实际出口。完整 [成功检查](user-guide.md#verify)。

停用时，热点设备回原 Wi-Fi；HTTP/PAC 关闭代理；静态网关恢复原地址或自动获取。完整 [恢复步骤](user-guide.md#stop)。
