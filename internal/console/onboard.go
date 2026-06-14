package console

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

// --- Onboarding: 只问代理端口，其它用推荐默认 ---
//
// 设计：小白用户 80% 的情况下只需要告诉网关 "把流量转发到哪"。
// 网关开关 / TUN / DNS / 规则模式 / 广告拦截 全部用推荐默认值，
// 在主菜单里可以随时调整。
func (c *consoleUI) onboard(ctx context.Context) error {
	c.banner("欢迎使用 lan-proxy-gateway · 首次配置")

	// 检测网络，展示给用户看（让人心里有数，但不用选）
	if err := c.app.Gateway.Detect(); err == nil {
		info := c.app.Gateway.Info()
		fmt.Fprintf(c.out, "  已检测到你的网络：\n")
		fmt.Fprintf(c.out, "    默认接口  %s\n", info.Interface)
		fmt.Fprintf(c.out, "    本机 IP   %s\n", info.IP)
		fmt.Fprintf(c.out, "    路由器 IP %s\n\n", info.Gateway)
	}

	// 推荐默认（主菜单里可随时改）
	c.app.Cfg.Gateway.Enabled = true
	c.app.Cfg.Gateway.TUN.Enabled = true
	c.app.Cfg.Gateway.DNS.Enabled = true
	c.app.Cfg.Traffic.Mode = config.ModeRule
	c.app.Cfg.Traffic.Adblock = true

	okC.Fprintln(c.out, "  已为你启用推荐配置：")
	fmt.Fprintln(c.out, "    ✓ 局域网共享网关")
	fmt.Fprintln(c.out, "    ✓ TUN 虚拟网卡  【必开】")
	warnC.Fprintln(c.out, "       关了 TUN 的话，Switch/PS5/Apple TV 改了网关也只会被【傻路由】转发，")
	warnC.Fprintln(c.out, "       它们照样被墙。TUN 才是真正让流量走代理的关键。")
	warnC.Fprintln(c.out, "       只有"+dimC.Sprint("手机/电脑能手动填代理服务器时")+"才能关。")
	fmt.Fprintln(c.out, "    ✓ DNS 代理（端口 53）")
	fmt.Fprintln(c.out, "    ✓ 规则模式（国内直连 + 国外代理）")
	fmt.Fprintln(c.out, "    ✓ 广告拦截")
	dimC.Fprintln(c.out, "    （以上都能在主菜单→分流 & 规则 里随时改）")
	fmt.Fprintln(c.out)

	// 唯一需要用户决策的：代理端口来源
	titleC.Fprintln(c.out, "  只剩一件事：把流量转发到哪里？")
	fmt.Fprintln(c.out, "    1) 单点代理        (填 主机+端口；本机 Clash Verge / 远程机场的单个节点都走这个)")
	fmt.Fprintln(c.out, "    2) 机场订阅        (粘一个订阅 URL，网关自己抓节点列表)")
	fmt.Fprintln(c.out, "    3) 本地配置文件    (指向一个 .yaml；格式和机场订阅一致，只是本地)")
	fmt.Fprintln(c.out, "    4) 暂不配置        (全部走直连，以后再来)")
	fmt.Fprintln(c.out)
	choice := c.ask("请选择 1-4", "1")
	switch choice {
	case "2":
		c.configureSubscription()
	case "3":
		c.configureFile()
	case "4":
		c.app.Cfg.Source.Type = config.SourceTypeNone
	default:
		c.configureSingle()
	}

	if err := c.app.Save(); err != nil {
		badC.Fprintf(c.out, "保存配置失败: %v\n", err)
		return err
	}
	okC.Fprintln(c.out, "\n✔ 配置已保存到 "+c.app.Paths.ConfigFile)
	return nil
}

// --- Source-type configurators ---

// configureSingle 是「单点代理」入口，合并以前的 external + remote。
// 填了用户名就存 SourceTypeRemote（有认证），否则存 SourceTypeExternal（无认证），
// 两种 type 底层 materialize 出来的 mihomo proxy 形态一致，只是认证字段的有无。
func (c *consoleUI) configureSingle() {
	// 从当前配置取已有值做默认（无论目前是 external 还是 remote）
	defServer, defPort, defKind, defUser, defPass := "127.0.0.1", 7890, "http", "", ""
	switch c.app.Cfg.Source.Type {
	case config.SourceTypeExternal:
		e := c.app.Cfg.Source.External
		defServer = firstNonEmpty(e.Server, defServer)
		defPort = firstNonZero(e.Port, defPort)
		defKind = firstNonEmpty(e.Kind, defKind)
	case config.SourceTypeRemote:
		r := c.app.Cfg.Source.Remote
		defServer = firstNonEmpty(r.Server, defServer)
		defPort = firstNonZero(r.Port, defPort)
		defKind = firstNonEmpty(r.Kind, defKind)
		defUser = r.Username
		defPass = r.Password
	}

	server := c.ask("  主机（本机代理就填 127.0.0.1）", defServer)
	portStr := c.ask("  端口", strconv.Itoa(defPort))
	port := defPort
	if p, err := strconv.Atoi(strings.TrimSpace(portStr)); err == nil && p > 0 {
		port = p
	}
	kind := strings.ToLower(c.ask("  类型 (http 或 socks5)", defKind))
	if kind != "socks5" {
		kind = "http"
	}
	user := c.ask("  用户名（不需要认证直接回车）", defUser)
	pass := defPass
	if user != "" {
		pass = c.ask("  密码", defPass)
	}

	if user == "" {
		c.app.Cfg.Source.Type = config.SourceTypeExternal
		c.app.Cfg.Source.External = config.ExternalProxy{
			Name:   firstNonEmpty(c.app.Cfg.Source.External.Name, "单点代理"),
			Server: server,
			Port:   port,
			Kind:   kind,
		}
	} else {
		c.app.Cfg.Source.Type = config.SourceTypeRemote
		c.app.Cfg.Source.Remote = config.RemoteProxy{
			Name:     firstNonEmpty(c.app.Cfg.Source.Remote.Name, "单点代理"),
			Server:   server,
			Port:     port,
			Kind:     kind,
			Username: user,
			Password: pass,
		}
	}
}

// configureScript 分三路：预设（链式代理向导）/ 自定义 .js 路径 / 清除。
// 预设会把用户填的住宅 IP 实例化到 workdir 的模板脚本，不用自己写 JS。
func (c *consoleUI) configureScript() {
	for {
		c.banner("全局扩展脚本")
		// 当前状态
		state := "未配置"
		if c.app.Cfg.Source.ChainResidential != nil {
			r := c.app.Cfg.Source.ChainResidential
			state = fmt.Sprintf("预设 · 链式代理（住宅 IP %s:%d %s）", r.Server, r.Port, r.Kind)
		} else if c.app.Cfg.Source.ScriptPath != "" {
			state = "自定义脚本 · " + c.app.Cfg.Source.ScriptPath
		}
		fmt.Fprintf(c.out, "  当前：%s\n\n", state)
		fmt.Fprintln(c.out, "  1  预设 · 链式代理（住宅 IP 落地）")
		dimC.Fprintln(c.out, "      填订阅节点先走机场，再链到住宅 IP 出国；AI 域名自动走住宅 IP")
		fmt.Fprintln(c.out, "  2  自定义 .js 文件路径")
		dimC.Fprintln(c.out, "      Clash Verge Rev 同款 main(config) 脚本")
		fmt.Fprintln(c.out, "  3  清除当前脚本")
		fmt.Fprintln(c.out)
		titleC.Fprintln(c.out, "  ── 操作 ── 0 返回（或按 Q）")
		switch strings.ToLower(c.prompt("选择：> ")) {
		case "1":
			c.configureScriptResidentialChain()
			return
		case "2":
			c.configureScriptCustomPath()
			return
		case "3":
			c.app.Cfg.Source.ScriptPath = ""
			c.app.Cfg.Source.ChainResidential = nil
			okC.Fprintln(c.out, "  已清除全局扩展脚本")
			return
		case "0", "q", "":
			return
		default:
			warnC.Fprintln(c.out, "无效选项")
		}
	}
}

// configureScriptResidentialChain 引导用户填住宅 IP 节点字段，然后写入配置。
// 脚本本身会在 render 时从内嵌模板实例化到 workdir，用户不用碰 JS 代码。
func (c *consoleUI) configureScriptResidentialChain() {
	cur := c.app.Cfg.Source.ChainResidential
	defName, defKind, defServer, defPort, defUser, defPass := "🏠 住宅IP", "socks5", "", 0, "", ""
	if cur != nil {
		defName = firstNonEmpty(cur.Name, defName)
		defKind = firstNonEmpty(cur.Kind, defKind)
		defServer = cur.Server
		defPort = cur.Port
		defUser = cur.Username
		defPass = cur.Password
	}

	fmt.Fprintln(c.out, "\n  请填写住宅 IP 落地节点（最终流量经机场 → 住宅 IP 出国）：")
	name := c.ask("  节点名称", defName)
	kind := strings.ToLower(c.ask("  类型 (http / socks5)", defKind))
	if kind != "http" && kind != "socks5" {
		kind = "socks5"
	}
	server := c.ask("  服务器地址", defServer)
	if server == "" {
		warnC.Fprintln(c.out, "  服务器地址不能为空，取消")
		return
	}
	portStr := c.ask("  端口", strconv.Itoa(firstNonZero(defPort, 443)))
	port, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil || port <= 0 || port > 65535 {
		warnC.Fprintln(c.out, "  端口无效，取消")
		return
	}
	user := c.ask("  用户名（无需认证回车）", defUser)
	pass := defPass
	if user != "" {
		pass = c.ask("  密码", defPass)
	}

	c.app.Cfg.Source.ChainResidential = &config.ChainResidentialConfig{
		Name:        name,
		Kind:        kind,
		Server:      server,
		Port:        port,
		Username:    user,
		Password:    pass,
		DialerProxy: "🛫 AI起飞节点",
	}
	// 预设会接管 ScriptPath，清掉用户自定义路径避免混乱。
	c.app.Cfg.Source.ScriptPath = ""
	okC.Fprintf(c.out, "  ✓ 已保存链式代理预设（%s:%d %s），下次 start/reload 生效\n", server, port, kind)
}

// configureScriptCustomPath 让用户填自定义 .js 路径（高级用法）。
func (c *consoleUI) configureScriptCustomPath() {
	fmt.Fprintln(c.out, "\n  填入 .js 绝对路径。直接回车 = 清除。")
	path := strings.TrimSpace(c.ask("  脚本路径", c.app.Cfg.Source.ScriptPath))
	if path == "" {
		c.app.Cfg.Source.ScriptPath = ""
		c.app.Cfg.Source.ChainResidential = nil
		okC.Fprintln(c.out, "  已清除全局扩展脚本")
		return
	}
	if _, err := os.Stat(path); err != nil {
		warnC.Fprintf(c.out, "  ⚠ 找不到 %s: %v\n", path, err)
		if !c.yesNo("  仍要保存这个路径？", false) {
			return
		}
	}
	c.app.Cfg.Source.ScriptPath = path
	c.app.Cfg.Source.ChainResidential = nil
	okC.Fprintf(c.out, "  已设置自定义脚本: %s\n", path)
}

func (c *consoleUI) configureSubscription() bool {
	oldType := c.app.Cfg.Source.Type
	oldURL := c.app.Cfg.Source.Subscription.URL
	oldName := c.app.Cfg.Source.Subscription.Name

	c.app.Cfg.Source.Type = config.SourceTypeSubscription
	s := &c.app.Cfg.Source.Subscription
	s.URL = c.ask("  订阅 URL", s.URL)
	s.Name = c.ask("  订阅名称", firstNonEmpty(s.Name, "subscription"))

	return oldType != c.app.Cfg.Source.Type || oldURL != s.URL || oldName != s.Name
}

func (c *consoleUI) configureFile() bool {
	oldType := c.app.Cfg.Source.Type
	oldPath := c.app.Cfg.Source.File.Path

	c.app.Cfg.Source.Type = config.SourceTypeFile
	c.app.Cfg.Source.File.Path = c.ask("  本地配置文件绝对路径 (Clash/mihomo YAML)", c.app.Cfg.Source.File.Path)

	return oldType != c.app.Cfg.Source.Type || oldPath != c.app.Cfg.Source.File.Path
}
