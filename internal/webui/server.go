package webui

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Server 包装 *app.App 暴露 HTTP 接口。
// 调用方一般是 cmd/start.go：start 时 Server.Start(ctx)，stop 时 Server.Shutdown(ctx)。
//
// 不依赖具体 App 类型 —— 通过 Controller 接口把 app 包注入进来，避免 internal/webui
// 反向 import internal/app 造成循环（internal/app 在内部 import 了 engine/gateway/
// platform 等等，webui 通过 Controller 拿到回调即可）。
type Server struct {
	addr   string // 实际监听地址（用于 logging）
	token  string // Bearer 鉴权 token；空表示禁用鉴权（仅用于测试）
	srv    *http.Server
	mu     sync.Mutex
	closed bool
}

// Controller 是 webui 与 app 之间的薄接口。app 包实现这个接口注入进 Server。
// 没把 *app.App 直接暴露出去是为了：
//  1. 让 internal/webui 不 import internal/app（webui 是叶子包，方便单测）
//  2. Controller 描述了"webui 需要后端做什么"，是一份明确的能力清单
type Controller interface {
	Snapshot() Snapshot
	SetGatewayMode(ctx context.Context, mode string) error
	SetTUNEnabled(ctx context.Context, enabled bool) error
	SetTrafficMode(ctx context.Context, mode string) error
	SetAdblock(ctx context.Context, enabled bool) error
	SetDNSEnabled(ctx context.Context, enabled bool) error
	SetAutoGroups(ctx context.Context, enabled bool) error
	SetRulesets(ctx context.Context, r Rulesets) error
	SetCustomRules(ctx context.Context, rs CustomRules) error
	SetSource(ctx context.Context, src SourcePayload) error
	SetScript(ctx context.Context, s ScriptPayload) error
	SetPorts(ctx context.Context, p Ports) error
	SetProxyService(ctx context.Context, p ProxyServicePayload) error
	CheckUpdate(ctx context.Context) (UpdateInfo, error)
	RunUpdate(ctx context.Context) error
	Reload(ctx context.Context) error
	Restart(ctx context.Context) error
}

// Ports 给前端调整 mihomo / WebUI 端口的载荷。0 = 不动那一项。
// 写入后需要完整重启（mihomo 启动时才读端口）。
type Ports struct {
	Mixed int `json:"mixed,omitempty"`
	Redir int `json:"redir,omitempty"`
	API   int `json:"api,omitempty"`
	WebUI int `json:"web_ui,omitempty"`
	DNS   int `json:"dns,omitempty"`
}

// New 构造一个 Server。addr 例如 ":19091"；空字符串表示禁用 webui（不监听）。
// token 是 /api/* 的 Bearer 鉴权 token，空字符串则不鉴权（**仅供测试**，生产必填）。
func New(addr, token string, ctrl Controller) *Server {
	if ctrl == nil {
		panic("webui.New: nil controller")
	}
	s := &Server{addr: addr, token: token}
	mux := http.NewServeMux()
	s.routes(mux, ctrl)
	s.srv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return s
}

// requireToken 是 /api/* 路径的 Bearer 鉴权中间件。
//
// 设计：
//   - GET / 静态资源不在这里包，开放访问（页面要先加载才能取 token）
//   - 本机 loopback 访问直接放行，方便在 gateway 主机上打开 http://127.0.0.1:19091/
//   - 非本机 /api/* 调用必须带 `Authorization: Bearer <token>` 或 `X-Gateway-Token: <token>`
//   - 用 `crypto/subtle.ConstantTimeCompare` 防计时侧信道
//   - 401 返回 JSON `{"error":"unauthorized"}` 让前端能识别并跳到"请用带 token 的 URL"提示
//
// 不鉴权 GET /api/ping（活探），方便 `gateway webui` 子命令探活用。
func (s *Server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" {
			// token 空 = 测试模式，放行。生产路径在 cmd/start.go 必须传非空 token。
			next(w, r)
			return
		}
		if requestFromLoopback(r) {
			next(w, r)
			return
		}
		got := strings.TrimSpace(r.Header.Get("X-Gateway-Token"))
		if got == "" {
			authz := strings.TrimSpace(r.Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				got = strings.TrimSpace(authz[len("Bearer "):])
			}
		}
		// fallback：允许 URL query ?_token=xxx，方便手测 / curl
		if got == "" {
			got = r.URL.Query().Get("_token")
		}
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "unauthorized · 请使用 CLI 启动时显示的完整 URL（含 #token=）访问",
			})
			return
		}
		next(w, r)
	}
}

func requestFromLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// Addr 返回实际监听地址（含端口），Start 之前是配置值，Start 之后会变成 Listener.Addr。
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// Start 在新 goroutine 跑 http.Server，函数本身立即返回。
// 监听失败返回 error；成功后 fatal 错误通过日志暴露（callers 不应阻塞在这）。
// 端口 0 或空 addr 视作 disabled，返回 nil 且不监听。
func (s *Server) Start(_ context.Context, logf func(string, ...any)) error {
	if s.addr == "" || s.addr == ":0" {
		if logf != nil {
			logf("webui disabled (runtime.ports.web_ui = 0)")
		}
		return nil
	}
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("webui listen %s: %w", s.addr, err)
	}
	s.mu.Lock()
	s.addr = ln.Addr().String()
	s.mu.Unlock()

	go func() {
		if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			if logf != nil {
				logf("webui serve error: %v", err)
			}
		}
	}()
	if logf != nil {
		logf("webui listening on http://%s", humanAddr(s.addr))
	}
	return nil
}

// Shutdown 优雅关闭 HTTP server。重复调用安全。
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

// humanAddr 把 "[::]:19091" / "0.0.0.0:19091" 之类换成更友好的本地访问 URL。
func humanAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "localhost"
	}
	// IPv6 包一层方括号
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		host = "[" + host + "]"
	}
	return host + ":" + port
}

// staticFileSystem 把 embed.FS 里的 static/* 暴露成 http 根目录的内容。
func staticFileSystem() http.FileSystem {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// 编译期已经 embed 进去了，运行期不该失败
		panic(fmt.Sprintf("webui: subFS failed: %v", err))
	}
	return http.FS(sub)
}

// PortFromInt 把 int 端口转成 host:port 形式，0 表示禁用。
func PortFromInt(port int) string {
	if port <= 0 {
		return ""
	}
	return "0.0.0.0:" + strconv.Itoa(port)
}
