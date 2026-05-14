package engine

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	embedpkg "github.com/tght/lan-proxy-gateway/embed"
	configpkg "github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/source"
)

// deployWebUI 把 go:embed 进来的 embed/webui/* 释放到 workdir/ui/ 下。
// 覆盖已有文件（用户升级 binary 后 UI 也同步升），但保留 workdir/ui 里
// 用户额外放的文件（例如 metacubexd 整个目录）。
//
// embed.FS 的路径永远是 forward-slash（`webui/_nuxt/xxx.js`），不能用
// filepath.Rel —— 在 Windows 上 filepath.Rel 按反斜杠逻辑处理，混合分隔
// 符会产出像 `..\webui\_nuxt\xxx.js` 的错结果，悄悄把文件写到 workdir
// 外，/ui/ 就成了空目录。用 path.Clean + strings.TrimPrefix 明确走
// forward-slash 分支，最后 filepath.FromSlash 转成本地分隔符再落盘。
func deployWebUI(workDir string) error {
	const root = "webui"
	uiRoot := filepath.Join(workDir, "ui")
	return fs.WalkDir(embedpkg.WebUI, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return os.MkdirAll(uiRoot, 0o755)
		}
		rel := strings.TrimPrefix(path.Clean(p), root+"/")
		dst := filepath.Join(uiRoot, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := fs.ReadFile(embedpkg.WebUI, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
}

// Engine wraps the mihomo process + its REST API. One instance per running gateway.
type Engine struct {
	bin      string
	workdir  string
	cacheDir string // optional, for geodata caching across reinstalls
	proc     *process
	api      *Client
	// apiPort 用来做跨 uid 的存活探测。sudo gateway 写的 mihomo.pid 在 /var/root
	// 下，普通 gateway 读不到会误判「未启动」。Attach 时把 cfg.Runtime.Ports.API
	// 灌进来，Running() 在 pid 路径失效时回退到端口探测，两种启动方式看到的状
	// 态就一致了。
	apiPort   int
	mixedPort int
}

// New returns an Engine configured to run `bin` with its working directory.
// cacheDir is optional (pass "" to disable geodata caching).
//
// proc is pre-created so Running() can detect a pre-existing background mihomo
// left over from a prior `gateway` session that exited without calling Stop().
func New(bin, workdir, cacheDir string) *Engine {
	e := &Engine{
		bin:      bin,
		workdir:  workdir,
		cacheDir: cacheDir,
		api:      NewClient(""), // baseURL filled in Start()/Attach()
	}
	e.proc = newProcess(bin, workdir, filepath.Join(workdir, "mihomo.log"))
	return e
}

// Attach wires the API client to an already-running mihomo. Returns true if a
// live process was found. This is the reattach path: the prior `gateway`
// process exited, left mihomo as an orphan, and the new process wants to pick
// up where it left off.
//
// 先写入 apiPort 再查 Running()：Running() 在找不到 pidfile 时会用 apiPort 做
// 端口探测，这样即便上一次是 sudo 启的（pid 写在 /var/root），本次以普通用户
// 起也能认出 mihomo 还在后台跑。
func (e *Engine) Attach(cfg *configpkg.Config) bool {
	e.apiPort = cfg.Runtime.Ports.API
	e.mixedPort = cfg.Runtime.Ports.Mixed
	e.api.baseURL = fmt.Sprintf("http://127.0.0.1:%d", cfg.Runtime.Ports.API)
	e.api.secret = cfg.Runtime.APISecret
	return e.Running()
}

// Workdir returns the working directory where the rendered config lives.
func (e *Engine) Workdir() string { return e.workdir }

// ConfigPath returns the path to the rendered mihomo config.yaml.
func (e *Engine) ConfigPath() string { return filepath.Join(e.workdir, "config.yaml") }

// API exposes the HTTP client (already configured to hit the running mihomo).
func (e *Engine) API() *Client { return e.api }

// Start renders the config, launches mihomo, and waits briefly for the API to come up.
func (e *Engine) Start(ctx context.Context, cfg *configpkg.Config) error {
	cfg = configpkg.EffectiveRuntimeConfig(cfg)
	return e.startRendered(ctx, cfg, nil)
}

func (e *Engine) startRendered(ctx context.Context, cfg *configpkg.Config, rendered []byte) error {
	if e.bin == "" {
		return fmt.Errorf("未找到 mihomo 二进制，请先运行 `gateway install`")
	}
	// If mihomo is already alive (from a prior session we attached to, or a
	// race in this session), treat Start as a no-op after wiring up the API.
	// Without this, preflight below would flag OUR OWN mihomo as a port
	// conflict and the user would be prompted to kill it.
	if e.Running() {
		e.apiPort = cfg.Runtime.Ports.API
		e.mixedPort = cfg.Runtime.Ports.Mixed
		e.api.baseURL = fmt.Sprintf("http://127.0.0.1:%d", cfg.Runtime.Ports.API)
		e.api.secret = cfg.Runtime.APISecret
		return nil
	}
	if err := os.MkdirAll(e.workdir, 0o755); err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}

	// Preflight: check port conflicts so we fail fast with a clear error.
	checks := []PortCheck{
		{Label: "mihomo mixed (HTTP+SOCKS5)", Port: cfg.Runtime.Ports.Mixed, Bind: "0.0.0.0"},
		{Label: "mihomo API", Port: cfg.Runtime.Ports.API, Bind: "127.0.0.1"},
	}
	if cfg.Gateway.DNS.Enabled {
		checks = append(checks, PortCheck{Label: "DNS", Port: cfg.Gateway.DNS.Port, Bind: "0.0.0.0"})
	}
	if err := CheckPorts(checks); err != nil {
		return err
	}

	// 确保 GeoIP / GeoSite 文件齐全，避免 mihomo 启动时卡在下载。
	// 正常路径（install 已跑过）会命中缓存/workdir，秒过；
	// 冷启动（workdir 被清掉）会静默下载。
	upstream := localUpstreamURL(cfg)
	_ = mihomo.EnsureGeodata(e.workdir, e.cacheDir, upstream, nil)

	data := rendered
	if data == nil {
		var err error
		data, err = Render(ctx, cfg, e.workdir)
		if err != nil {
			return err
		}
	}
	if err := os.WriteFile(e.ConfigPath(), data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	logPath := filepath.Join(e.workdir, "mihomo.log")
	_ = os.Truncate(logPath, 0) // start fresh so tail-on-fail shows only this run
	e.proc = newProcess(e.bin, e.workdir, logPath)
	if err := e.proc.Start(); err != nil {
		return fmt.Errorf("启动 mihomo 失败: %w", err)
	}
	e.apiPort = cfg.Runtime.Ports.API
	e.mixedPort = cfg.Runtime.Ports.Mixed
	e.api.baseURL = fmt.Sprintf("http://127.0.0.1:%d", cfg.Runtime.Ports.API)
	e.api.secret = cfg.Runtime.APISecret
	if err := e.api.WaitReady(ctx, 10*time.Second); err != nil {
		tail := TailLog(logPath, 20)
		_ = e.proc.Stop()
		if tail != "" {
			return fmt.Errorf("mihomo 启动超时（API 未就绪）。\n\nmihomo 日志最后 20 行：\n%s\n\n完整日志: %s", tail, logPath)
		}
		return fmt.Errorf("mihomo 启动超时且无日志输出，检查二进制权限: %s", e.bin)
	}
	return nil
}

// Stop kills the mihomo process. Safe to call if never started.
func (e *Engine) Stop() error {
	if e.proc == nil {
		return nil
	}
	return e.proc.Stop()
}

// Reload 重新渲染 config 并重启 mihomo。
//
// 以前走 API /configs reload（快、不断流），但 mihomo 的 API reload **不更新
// 顶级项** —— external-ui / external-controller / tun / dns.listen 这些都要
// 进程重启才生效。用户从菜单点「重启」后发现 UI 不通、端口没换，体验很差。
// 统一走 Stop+Start 简单可靠，代价是 LAN 设备断流 1-2 秒。
func (e *Engine) Reload(ctx context.Context, cfg *configpkg.Config) error {
	cfg = configpkg.EffectiveRuntimeConfig(cfg)
	if !e.Running() {
		return e.Start(ctx, cfg)
	}
	data, err := renderWithOptions(ctx, cfg, e.workdir, renderOptions{
		subscriptionProxyURL: source.LocalMixedProxyURL(e.mixedPort),
	})
	if err != nil {
		return err
	}
	_ = e.Stop()
	return e.startRendered(ctx, cfg, data)
}

// Running reports whether mihomo is alive.
//
// 两段判定：先看进程 handle（自己 fork 出来的子进程 + 同 uid 写的 pidfile）；
// 拿不到再用 apiPort 做 TCP 探测，覆盖「上一次 sudo 启的，这次普通用户接管」
// 这种 uid 不同导致 pidfile 不可读的场景。
func (e *Engine) Running() bool {
	if e.proc != nil && e.proc.Alive() {
		return true
	}
	return e.apiPort > 0 && probeAPIPort(e.apiPort)
}

// LogPath returns the path to the mihomo log file.
func (e *Engine) LogPath() string { return filepath.Join(e.workdir, "mihomo.log") }

// localUpstreamURL returns a proxy URL if the user's source is an external
// proxy — so geodata downloads can route through it when direct is blocked.
func localUpstreamURL(cfg *configpkg.Config) string {
	if cfg.Source.Type != configpkg.SourceTypeExternal {
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
