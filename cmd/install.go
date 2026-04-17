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

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "一键安装：下载内核 + 数据文件 + 配置向导 + 启动",
	RunE: func(cmd *cobra.Command, args []string) error {
		// install 会直接启动网关，所以需要 root。先提权，这样用户只输一次 sudo 密码。
		maybeElevate()

		plat := platform.Current()

		// Step 1: mihomo 内核
		color.Cyan("[1/4] 检查 mihomo 内核…")
		if _, err := plat.ResolveMihomoPath(""); err != nil {
			color.Yellow("  未检测到 mihomo，开始下载…")
			dest := defaultInstallDir()
			inst := mihomopkg.Installer{DestDir: dest}
			path, err := inst.Install()
			if err != nil {
				return fmt.Errorf("下载 mihomo 失败: %w", err)
			}
			color.Green("  ✓ mihomo 已安装: %s", path)
		} else {
			color.Green("  ✓ mihomo 已就绪")
		}

		// Step 2: 配置（走向导或加载已有）
		color.Cyan("\n[2/4] 配置代理网关…")
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
		color.Cyan("\n[3/4] 准备 GeoIP / GeoSite 数据文件…")
		upstream := localUpstreamURL(a.Cfg)
		if err := mihomopkg.EnsureGeodata(a.Paths.MihomoDir, a.Paths.CacheDir, upstream, func(format string, args ...any) {
			fmt.Printf("  "+format+"\n", args...)
		}); err != nil {
			color.Yellow("  ⚠ %v", err)
		}
		config.ReclaimToSudoUser(a.Paths.Root)
		config.ReclaimToSudoUser(a.Paths.CacheDir)

		// Step 4: 启动
		color.Cyan("\n[4/4] 启动网关…")
		startErr := a.Start(cmd.Context())
		if startErr != nil {
			color.Red("  ✗ 启动失败:")
			fmt.Println(indent("    ", fmt.Sprintf("%v", startErr)))
			color.Yellow("\n不用怕，已把你带到【主菜单】，在里面就能修。")
			color.New(color.Faint).Println("  • 选 2 流量控制 → 可关掉 DNS（最常见的端口冲突）")
			color.New(color.Faint).Println("  • 修好后选 4 生命周期 → 1 启动  即可再次尝试")
			color.New(color.Faint).Println("  • 任何时候按 Q 退出，不想改了直接关窗口也行")
			fmt.Println()
			// 进入主菜单让用户能原地修；不是把错误抛出去直接退出。
			return console.Run(cmd.Context(), a)
		}
		color.Green("  ✓ 网关已启动")
		fmt.Println()
		color.Cyan("设备接入指引：")
		fmt.Println("  把其它设备（Switch / PS5 / Apple TV / 手机）的")
		fmt.Printf("    网关 + DNS → %s\n", a.Status().Gateway.LocalIP)
		fmt.Println("  保存并重连 Wi-Fi 即可。")
		fmt.Println()
		color.New(color.Faint).Printf("Ctrl+C 停止；日志 %s\n", a.Engine.LogPath())

		// 保持前台运行，等 Ctrl+C
		waitForSignal()
		color.Yellow("\n正在停止…")
		return a.Stop()
	},
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
