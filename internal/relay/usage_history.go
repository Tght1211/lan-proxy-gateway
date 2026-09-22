package relay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// DailyUsage stores only device/ingress byte totals, never destination history.
type DailyUsage struct {
	Date        string    `json:"date"`
	Device      string    `json:"device"`
	Ingress     string    `json:"ingress"`
	Up          int64     `json:"up"`
	Down        int64     `json:"down"`
	Connections int64     `json:"connections"`
	LastSeen    time.Time `json:"last_seen"`
}
type usageHistory struct {
	mu     sync.Mutex
	saveMu sync.Mutex
	path   string
	rows   map[string]*DailyUsage
}

func (t *Tracker) EnableHistory(path string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	rows := []DailyUsage{}
	if err == nil {
		if err = json.Unmarshal(data, &rows); err != nil {
			return fmt.Errorf("read usage history: %w", err)
		}
	}
	h := &usageHistory{path: path, rows: map[string]*DailyUsage{}}
	for _, row := range rows {
		if _, err := time.Parse("2006-01-02", row.Date); err != nil || row.Up < 0 || row.Down < 0 {
			return fmt.Errorf("invalid usage history")
		}
		copy := row
		h.rows[row.Date+"|"+row.Device+"|"+row.Ingress] = &copy
	}
	t.history = h
	return nil
}
func (t *Tracker) recordUsage(c *TrackedConn, up, down, connections int64, now time.Time) {
	h := t.history
	if h == nil {
		return
	}
	day := now.Local().Format("2006-01-02")
	key := day + "|" + c.srcIP + "|" + c.ingress
	h.mu.Lock()
	defer h.mu.Unlock()
	row := h.rows[key]
	if row == nil {
		row = &DailyUsage{Date: day, Device: c.srcIP, Ingress: c.ingress}
		h.rows[key] = row
	}
	row.Up += up
	row.Down += down
	row.Connections += connections
	row.LastSeen = now
}
func (t *Tracker) UsageHistory() []DailyUsage {
	h := t.history
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	rows := make([]DailyUsage, 0, len(h.rows))
	for _, row := range h.rows {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date < rows[j].Date
		}
		if rows[i].Device != rows[j].Device {
			return rows[i].Device < rows[j].Device
		}
		return rows[i].Ingress < rows[j].Ingress
	})
	return rows
}
func (t *Tracker) SaveHistory() error {
	h := t.history
	if h == nil {
		return nil
	}
	h.saveMu.Lock()
	defer h.saveMu.Unlock()
	data, err := json.Marshal(t.UsageHistory())
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(h.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(h.path), ".usage-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), h.path)
}
