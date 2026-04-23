// Package geoip 把 IP 翻译成国家码 + 国旗 emoji。仪表盘显示落地节点位置和
// 设备地理分布时用。
//
// 数据源是 mihomo 已经下载到 workdir 的 country.mmdb（MaxMind GeoLite2 格式）。
// 没 mmdb 文件或打不开时所有查询安静返回零值，调用方只要兜住空字符串就行。
package geoip

import (
	"fmt"
	"net"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

// DB 是一个线程安全的 mmdb 句柄。零值可用但所有查询都返回空。
type DB struct {
	mu sync.RWMutex
	r  *maxminddb.Reader
}

// Open 打开 country.mmdb。文件不存在 / 损坏都返回 error，DB 可继续零值使用
// （Lookup 返回空值），调用方通常忽略错误 + 记一次 log 即可。
func Open(path string) (*DB, error) {
	r, err := maxminddb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open mmdb %s: %w", path, err)
	}
	return &DB{r: r}, nil
}

// Close 释放 mmap。零值 DB 调用安全。
func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.r == nil {
		return nil
	}
	err := d.r.Close()
	d.r = nil
	return err
}

// Lookup 返回 ISO-3166-1 alpha-2 国家码（例如 "HK"）和对应的 regional
// indicator emoji（🇭🇰）。
//
// 特殊返回：
//   - 私有 / 回环 IP（LAN 设备、127.0.0.1）→ "", "🏠"
//   - mmdb 没打开 / 查不到 → "", ""
func (d *DB) Lookup(ip net.IP) (country, flag string) {
	if ip == nil {
		return "", ""
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return "", "🏠"
	}
	if d == nil {
		return "", ""
	}
	d.mu.RLock()
	r := d.r
	d.mu.RUnlock()
	if r == nil {
		return "", ""
	}
	var rec struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}
	if err := r.Lookup(ip, &rec); err != nil || rec.Country.ISOCode == "" {
		return "", ""
	}
	return rec.Country.ISOCode, FlagFor(rec.Country.ISOCode)
}

// LookupString 是 Lookup 的字符串入口（跳过 net.ParseIP 的重复模板）。
func (d *DB) LookupString(ip string) (country, flag string) {
	return d.Lookup(net.ParseIP(ip))
}

// FlagFor 把 ISO-3166-1 alpha-2 国家码转成国旗 emoji。非法码返回空串。
//
// 原理：国旗 emoji 就是两个 Regional Indicator Symbol Letter 字符拼起来，
// 每个 letter 是 U+1F1E6(🇦) + (上case字母-'A')。大小写不敏感。
func FlagFor(code string) string {
	if len(code) != 2 {
		return ""
	}
	a, b := code[0], code[1]
	if a >= 'a' && a <= 'z' {
		a -= 'a' - 'A'
	}
	if b >= 'a' && b <= 'z' {
		b -= 'a' - 'A'
	}
	if a < 'A' || a > 'Z' || b < 'A' || b > 'Z' {
		return ""
	}
	const base = 0x1F1E6 // Regional Indicator A
	return string([]rune{base + rune(a-'A'), base + rune(b-'A')})
}
