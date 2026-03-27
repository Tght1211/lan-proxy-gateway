package cmd

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/proxy"
	tmpl "github.com/tght/lan-proxy-gateway/internal/template"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

//go:embed webui/*
var webUIFS embed.FS

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "启动本地个性化配置页面",
	Run:   runUI,
}

func init() {
	rootCmd.AddCommand(uiCmd)
}

func runUI(cmd *cobra.Command, args []string) {
	cfgPath := resolveConfigPath()
	cfg := loadConfigOrDefault()
	listenAddr := cfg.UI.Listen
	if listenAddr == "" {
		listenAddr = config.DefaultConfig().UI.Listen
	}

	mux, err := newUIHTTPMux(cfgPath)
	if err != nil {
		ui.Error("页面初始化失败: %s", err)
		return
	}

	ui.Success("本地配置页面已启动: http://%s", listenAddr)
	ui.Info("按 Ctrl+C 退出")
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		ui.Error("页面服务启动失败: %s", err)
	}
}

func newUIHTTPMux(cfgPath string) (*http.ServeMux, error) {
	mux := http.NewServeMux()

	assets, err := fs.Sub(webUIFS, "webui")
	if err != nil {
		return nil, err
	}
	mux.Handle("/", http.FileServer(http.FS(assets)))

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cfg := loadConfigOrDefault()
			writeJSON(w, http.StatusOK, cfg)
		case http.MethodPut:
			var next config.Config
			if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
				writeError(w, http.StatusBadRequest, "无效配置 JSON")
				return
			}
			if next.ProxySource != "url" && next.ProxySource != "file" {
				writeError(w, http.StatusBadRequest, "proxy_source 必须是 url 或 file")
				return
			}
			if next.ProxySource == "url" && strings.TrimSpace(next.SubscriptionURL) == "" {
				writeError(w, http.StatusBadRequest, "subscription_url 不能为空")
				return
			}
			if next.ProxySource == "file" && strings.TrimSpace(next.ProxyConfigFile) == "" {
				writeError(w, http.StatusBadRequest, "proxy_config_file 不能为空")
				return
			}
			if err := config.Save(&next, cfgPath); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/apply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		cfg := loadConfigOrDefault()
		p := platform.New()
		dDir := ensureDataDir()
		iface, _ := p.DetectDefaultInterface()
		ip, _ := p.DetectInterfaceIP(iface)
		if cfg.ProxySource == "file" {
			providerFile := filepath.Join(dDir, "proxy_provider", cfg.SubscriptionName+".yaml")
			if _, err := proxy.ExtractProxies(cfg.ProxyConfigFile, providerFile); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		configPath := filepath.Join(dDir, "config.yaml")
		if err := tmpl.RenderTemplate(cfg, iface, ip, configPath); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "config_path": configPath})
	})

	mux.HandleFunc("/api/switch-best", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		cfg := loadConfigOrDefault()
		regions := append([]string{}, cfg.Regions.Include...)
		if len(regions) == 0 {
			writeError(w, http.StatusBadRequest, "未配置地区限制")
			return
		}

		groupName := r.URL.Query().Get("group")
		if groupName == "" {
			groupName = "Auto"
		}
		dryRun := r.URL.Query().Get("dry_run") == "1"

		apiURL := mihomo.FormatAPIURL("127.0.0.1", cfg.Ports.API)
		client := mihomo.NewClient(apiURL, cfg.APISecret)
		if !client.IsAvailable() {
			writeError(w, http.StatusBadRequest, "mihomo API 不可用")
			return
		}

		best, candidateCount, err := analyzeBestCandidate(client, cfg, groupName, regions)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if !dryRun {
			if err := client.SetProxyGroup(groupName, best.Name); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}

		delay := "unknown"
		if best.Delay != int(^uint(0)>>1) {
			delay = strconv.Itoa(best.Delay)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":              true,
			"dry_run":         dryRun,
			"group":           groupName,
			"best_node":       best.Name,
			"delay_ms":        delay,
			"matched_regions": best.MatchedCode,
			"candidate_count": candidateCount,
		})
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		cfg := loadConfigOrDefault()
		p := platform.New()
		running, pid, _ := p.IsRunning()
		apiURL := mihomo.FormatAPIURL("127.0.0.1", cfg.Ports.API)
		client := mihomo.NewClient(apiURL, cfg.APISecret)
		currentNode := ""
		if client.IsAvailable() {
			if pg, err := client.GetProxyGroup("Proxy"); err == nil {
				currentNode = pg.Now
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"running":      running,
			"pid":          pid,
			"current_node": currentNode,
			"regions":      cfg.Regions.Include,
		})
	})

	return mux, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": message})
}
