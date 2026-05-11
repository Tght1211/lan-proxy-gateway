package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/console"
	mihomopkg "github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/platform"
)

var (
	installReinstallMihomo bool
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "一键安装：下载内核 + 数据文件 + 配置向导 + 启动",
	RunE: func(cmd *cobra.Command, args []string) error {
		// install 会直接启动网关，所以需要 root。先提权，这样用户只输一次 sudo 密码。
		maybeElevate()

		plat := platform.Current()

		// Step 1: mihomo 内核
		// 走"是否要下载"决策的三个分支：
		//   1) 找不到 mihomo 二进制 → 下载
		//   2) 找到了但用户传了 --reinstall-mihomo → 强制覆盖下载（本版的主要
		//      用途：从 v3.4.1 (mihomo v1.18.6) 升上来的用户拿到 v1.19.24 以
		//      支持订阅里的 anytls 节点；issue #4）
		//   3) 找到了且用户没要求重装 → 跳过下载，沿用当前 binary
		color.Cyan("[1/5] 检查 mihomo 内核…")
		existing, lookupErr := plat.ResolveMihomoPath("")
		needDownload := lookupErr != nil || installReinstallMihomo
		if needDownload {
			if installReinstallMihomo && lookupErr == nil {
				color.Yellow("  --reinstall-mihomo 强制重新下载（覆盖现有 %s 到 %s）", existing, mihomopkg.PinnedMihomoVersion)
			} else {
				color.Yellow("  未检测到 mihomo，开始下载…")
			}
			dest := defaultInstallDir()
			inst := mihomopkg.Installer{
				DestDir: dest,
				Logf: func(format string, args ...any) {
					fmt.Printf("  "+format+"\n", args...)
				},
			}
			path, err := inst.Install()
			if err != nil {
				return fmt.Errorf("下载 mihomo 失败: %w\n    若所有镜像都超时，可设 HTTP_PROXY 环境变量或 GITHUB_MIRROR=<镜像前缀> 后重试", err)
			}
			color.Green("  ✓ mihomo 已安装: %s (%s)", path, mihomopkg.PinnedMihomoVersion)
		} else {
			color.Green("  ✓ mihomo 已就绪")
			color.New(color.Faint).Printf("    （本版本对应 mihomo %s。订阅里有 anytls / 新协议且报 unsupport proxy type 时，跑 %s 强制升级）\n",
				mihomopkg.PinnedMihomoVersion, elevatedCmd("install --reinstall-mihomo"))
		}

		// Step 2: 配置（走向导或加载已有）
		color.Cyan("\n[2/5] 配置代理网关…")
		paths, err := config.ResolvePaths()
		if err != nil {
			return err
		}
		if _, err := os.Stat(paths.ConfigFile); err == nil {
			color.Green("  ✓ 已存在配置 %s （跳过向导，如需重配请删除后再来）", paths.ConfigFile)
		}
		a, err := app.New()
		if err != nil {
			return err
		}
		if !a.Configured() {
			if err := console.RunOnboarding(cmd.Context(), a); err != nil {
				return err
			}
		}

		// Step 3: 预下载 GeoIP/GeoSite（避免 mihomo 启动时卡在下载）
		// 先查用户 cache 目录，再查 mihomo workdir，都没命中才下载；下载后存到 cache，
		// 这样下次 rm -rf ~/.config 再装也不会重新下载。
		color.Cyan("\n[3/5] 准备 GeoIP / GeoSite 数据文件…")
		upstream := localUpstreamURL(a.Cfg)
		if err := mihomopkg.EnsureGeodata(a.Paths.MihomoDir, a.Paths.CacheDir, upstream, func(format string, args ...any) {
			fmt.Printf("  "+format+"\n", args...)
		}); err != nil {
			color.Yellow("  ⚠ %v", err)
		}
		config.ReclaimToSudoUser(a.Paths.Root)
		config.ReclaimToSudoUser(a.Paths.CacheDir)

		// Step 4: 启动
		color.Cyan("\n[4/5] 启动网关…")
		startErr := a.Start(cmd.Context())
		if startErr != nil {
			color.Red("  ✗ 启动失败:")
			fmt.Println(indent("    ", fmt.Sprintf("%v", startErr)))
			// 错误已经带了具体的 "怎么修"（a.Start 的职责），不在这里重复一遍。
			// 只给一句导航：我把你带进主菜单了，修完在「启动 / 重启 / 停止」里再试。
			color.Yellow("\n已进入主菜单 — 按上方提示修好后，选 4（启动 / 重启 / 停止）重新启动。按 Q 随时退出。")
			fmt.Println()
			return console.Run(cmd.Context(), a)
		}
		color.Green("  ✓ 网关已启动")

		// Step 5: 开机自启（可选，默认 y）
		fmt.Println()
		color.Cyan("[5/5] 开机自启")
		fmt.Printf("  装上后，开机 / 重启会自动拉起 mihomo，不用再手动 %s。\n", elevatedCmd(""))
		if askYesNo("  要装开机自启吗？", true) {
			if binPath, err := os.Executable(); err != nil {
				color.Yellow("  ⚠ 无法定位当前可执行文件: %v（可稍后手动 %s）", err, elevatedCmd("service install"))
			} else if err := plat.InstallService(binPath); err != nil {
				color.Yellow("  ⚠ 安装服务失败: %v（可稍后手动 %s）", err, elevatedCmd("service install"))
			} else {
				color.Green("  ✓ 开机自启已启用")
			}
		} else {
			color.New(color.Faint).Printf("  跳过。以后想装：%s\n", elevatedCmd("service install"))
		}

		// 装完直接退出，mihomo 作为孤儿在后台跑。"可用就行了"。
		fmt.Println()
		color.Cyan("设备接入指引：")
		lanIP := a.Status().Gateway.LocalIP
		mixed := a.Cfg.Runtime.Ports.Mixed
		if runtime.GOOS == "windows" {
			// Windows 上 ConfigureNAT 是 no-op（家用版没 RRAS，ICS 强制
			// 192.168.137/24），"改网关" 走不通 —— 引导用户直接设 HTTP 代理。
			fmt.Println("  Windows 不支持 LAN 透明网关；请在设备上直接设 HTTP 代理：")
			fmt.Printf("    代理 → %s  端口 → %d  类型 → HTTP（或 SOCKS5）\n", lanIP, mixed)
			fmt.Println("    Android: Wi-Fi → 修改网络 → 高级 → 代理=手动")
			fmt.Println("    iOS:     Wi-Fi → 点 (i) → 配置代理=手动")
		} else {
			fmt.Println("  把其它设备（Switch / PS5 / Apple TV / 手机）的")
			fmt.Printf("    网关 + DNS → %s\n", lanIP)
			fmt.Println("  保存并重连 Wi-Fi 即可。")
		}
		fmt.Println()
		color.New(color.Faint).Printf("日志          %s\n", a.Engine.LogPath())
		color.New(color.Faint).Printf("调整配置      %-24s（主菜单：换源、切模式、广告拦截、端口…）\n", elevatedCmd(""))
		color.New(color.Faint).Printf("停止 mihomo   %s\n", elevatedCmd("stop"))
		return nil
	},
}

func init() {
	installCmd.Flags().BoolVar(&installReinstallMihomo, "reinstall-mihomo", false,
		"强制重新下载 mihomo 内核，覆盖已有版本（订阅里有 anytls / vless reality 等新协议但报 unsupport proxy type 时用）")
}

// askYesNo 读一行 y/n，空回车走默认。失败 fallback 到默认（non-TTY 下也不卡）。
func askYesNo(label string, def bool) bool {
	hint := "(Y/n)"
	if !def {
		hint = "(y/N)"
	}
	fmt.Printf("%s %s ", label, hint)
	var line string
	if _, err := fmt.Scanln(&line); err != nil {
		return def
	}
	switch line {
	case "":
		return def
	case "y", "Y", "yes", "YES":
		return true
	case "n", "N", "no", "NO":
		return false
	default:
		return def
	}
}

func indent(prefix, s string) string {
	out := ""
	for i, line := range splitLines(s) {
		if i > 0 {
			out += "\n"
		}
		out += prefix + line
	}
	return out
}

func splitLines(s string) []string {
	lines := []string{}
	cur := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	lines = append(lines, cur)
	return lines
}

func defaultInstallDir() string {
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("USERPROFILE"); dir != "" {
			return filepath.Join(dir, "AppData", "Local", "lan-proxy-gateway", "bin")
		}
	}
	if admin, _ := platform.Current().IsAdmin(); admin {
		switch runtime.GOOS {
		case "darwin", "linux":
			return "/usr/local/bin"
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
}

// localUpstreamURL returns http://server:port if the user's source is a local proxy,
// so the geodata downloader can go through it. Returns "" otherwise.
func localUpstreamURL(cfg *config.Config) string {
	if cfg.Source.Type != config.SourceTypeExternal {
		return ""
	}
	e := cfg.Source.External
	if e.Server == "" || e.Port == 0 {
		return ""
	}
	scheme := "http"
	if e.Kind == "socks5" {
		scheme = "socks5"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, e.Server, e.Port)
}

func selfName() string {
	if p, err := os.Executable(); err == nil {
		return filepath.Base(p)
	}
	return "gateway"
}
