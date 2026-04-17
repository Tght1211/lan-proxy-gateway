package egress

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"gopkg.in/yaml.v3"
)

const (
	ipEchoURL = "https://checkip.amazonaws.com"
	geoAPIURL = "https://ipwho.is"
)

var publicResolvers = []string{
	"223.5.5.5:53",
	"1.1.1.1:53",
	"8.8.8.8:53",
}

type Report struct {
	DetectedAt      time.Time
	ProbeSource     string
	ProxyExit       *GeoInfo
	AirportNode     *NodeInfo
	ResidentialExit *GeoInfo
}

type GeoInfo struct {
	IP      string
	Country string
	Region  string
	City    string
	ISP     string
}

type NodeInfo struct {
	Name     string
	Server   string
	Port     int
	Resolved string
	Location *GeoInfo
}

func Collect(cfg *config.Config, dataDir string, client *mihomo.Client) *Report {
	report := &Report{
		DetectedAt:  time.Now(),
		ProbeSource: "checkip.amazonaws.com + ipwho.is",
	}

	proxyURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Runtime.Ports.Mixed)

	if cfg.Extension.Mode == "chains" {
		report.AirportNode = detectAirportNode(cfg, dataDir, client)

		chainMode := "rule"
		if cfg.Extension.ResidentialChain != nil && cfg.Extension.ResidentialChain.Mode != "" {
			chainMode = cfg.Extension.ResidentialChain.Mode
		}

		if chainMode == "rule" {
			report.ProxyExit = geoLookupViaProxy(proxyURL)
		}
		report.ResidentialExit = residentialGeoLookup(proxyURL)
		return report
	}

	report.ProxyExit = geoLookupViaProxy(proxyURL)
	return report
}

func geoLookupViaProxy(proxyAddr string) *GeoInfo {
	client, err := newHTTPClient(proxyAddr)
	if err != nil {
		return nil
	}
	return geoLookup(client, geoAPIURL+"/")
}

func residentialGeoLookup(proxyAddr string) *GeoInfo {
	client, err := newHTTPClient(proxyAddr)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(http.MethodGet, ipEchoURL, nil)
	if err != nil {
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var ipBuf strings.Builder
	if _, err := io.Copy(&ipBuf, resp.Body); err != nil {
		return nil
	}

	ip := strings.TrimSpace(ipBuf.String())
	if ip == "" {
		return nil
	}

	return geoLookup(defaultClient(), geoAPIURL+"/"+ip)
}

func detectAirportNode(cfg *config.Config, dataDir string, client *mihomo.Client) *NodeInfo {
	if cfg.Extension.ResidentialChain == nil {
		return nil
	}

	nodeName := resolveAirportNodeName(cfg.Extension.ResidentialChain.AirportGroup, client)
	if nodeName == "" {
		return nil
	}

	server, port, ok := findNodeServer(filepath.Join(dataDir, "proxy_provider", cfg.Proxy.SubscriptionName+".yaml"), nodeName)
	if !ok {
		return &NodeInfo{Name: nodeName}
	}

	info := &NodeInfo{
		Name:   nodeName,
		Server: server,
		Port:   port,
	}

	resolved := resolveServerIP(server)
	if resolved == "" {
		return info
	}
	info.Resolved = resolved
	info.Location = geoLookup(defaultClient(), geoAPIURL+"/"+resolved)
	return info
}

func resolveAirportNodeName(airportGroup string, client *mihomo.Client) string {
	group := strings.TrimSpace(airportGroup)
	if group == "" {
		group = "Auto"
	}
	if client == nil {
		return group
	}
	pg, err := client.GetProxyGroup(group)
	if err != nil {
		return group
	}
	if strings.TrimSpace(pg.Now) != "" {
		return pg.Now
	}
	return group
}

func findNodeServer(path, nodeName string) (string, int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, false
	}

	var provider struct {
		Proxies []struct {
			Name   string `yaml:"name"`
			Server string `yaml:"server"`
			Port   int    `yaml:"port"`
		} `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(data, &provider); err != nil {
		return "", 0, false
	}

	for _, p := range provider.Proxies {
		if p.Name == nodeName {
			return p.Server, p.Port, p.Server != ""
		}
	}
	return "", 0, false
}

func resolveServerIP(server string) string {
	if server == "" {
		return ""
	}
	if ip := net.ParseIP(server); ip != nil {
		if !isFakeBenchmarkIP(ip) {
			return ip.String()
		}
	}

	if resolved := resolveViaPublicDNS(server); resolved != "" {
		return resolved
	}

	ips, err := net.LookupIP(server)
	if err == nil {
		if resolved := firstUsableIP(ips); resolved != "" {
			return resolved
		}
	}

	return ""
}

func resolveViaPublicDNS(server string) string {
	for _, dnsServer := range publicResolvers {
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				dialer := &net.Dialer{Timeout: 3 * time.Second}
				return dialer.DialContext(ctx, "udp", dnsServer)
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		ips, err := resolver.LookupIP(ctx, "ip", server)
		cancel()
		if err != nil {
			continue
		}

		if resolved := firstUsableIP(ips); resolved != "" {
			return resolved
		}
	}
	return ""
}

func firstUsableIP(ips []net.IP) string {
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil && !isFakeBenchmarkIP(v4) {
			return v4.String()
		}
	}
	for _, ip := range ips {
		if ip != nil && !isFakeBenchmarkIP(ip) {
			return ip.String()
		}
	}
	return ""
}

func isFakeBenchmarkIP(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	return v4[0] == 198 && (v4[1] == 18 || v4[1] == 19)
}

func geoLookup(client *http.Client, endpoint string) *GeoInfo {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var payload struct {
		Success    bool   `json:"success"`
		IP         string `json:"ip"`
		Country    string `json:"country"`
		Region     string `json:"region"`
		City       string `json:"city"`
		Connection struct {
			ISP string `json:"isp"`
		} `json:"connection"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}
	if !payload.Success {
		return nil
	}

	return &GeoInfo{
		IP:      payload.IP,
		Country: payload.Country,
		Region:  payload.Region,
		City:    payload.City,
		ISP:     payload.Connection.ISP,
	}
}

func newHTTPClient(proxyAddr string) (*http.Client, error) {
	parsed, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout: 6 * time.Second,
		Transport: &http.Transport{
			Proxy:               http.ProxyURL(parsed),
			DialContext:         (&net.Dialer{Timeout: 4 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			TLSHandshakeTimeout: 4 * time.Second,
		},
	}, nil
}

func defaultClient() *http.Client {
	return &http.Client{Timeout: 6 * time.Second}
}

func (g *GeoInfo) Summary() string {
	if g == nil {
		return "探测失败"
	}

	var areaParts []string
	for _, part := range []string{g.Country, g.Region, g.City} {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if len(areaParts) == 0 || areaParts[len(areaParts)-1] != part {
			areaParts = append(areaParts, part)
		}
	}

	summary := g.IP
	if len(areaParts) > 0 {
		summary += " · " + strings.Join(areaParts, " / ")
	}
	if strings.TrimSpace(g.ISP) != "" {
		summary += " · " + g.ISP
	}
	return summary
}

func (n *NodeInfo) Summary() string {
	if n == nil {
		return "未识别"
	}

	parts := []string{n.Name}
	if n.Location != nil {
		parts = append(parts, n.Location.AreaSummary())
	} else if n.Resolved != "" {
		parts = append(parts, n.Resolved)
	} else if n.Server != "" {
		parts = append(parts, n.Server)
	}
	return strings.Join(filterEmpty(parts), " · ")
}

func (g *GeoInfo) AreaSummary() string {
	if g == nil {
		return ""
	}

	var areaParts []string
	for _, part := range []string{g.Country, g.Region, g.City} {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if len(areaParts) == 0 || areaParts[len(areaParts)-1] != part {
			areaParts = append(areaParts, part)
		}
	}
	if strings.TrimSpace(g.ISP) != "" {
		areaParts = append(areaParts, g.ISP)
	}
	return strings.Join(areaParts, " / ")
}

func filterEmpty(values []string) []string {
	var out []string
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
