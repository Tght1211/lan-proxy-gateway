package console

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

const nodeListPreviewLimit = 20

type nodeListMode int

const (
	nodeListPreview nodeListMode = iota
	nodeListCompact
	nodeListVerbose
)

func sortProxyNodes(all []string, delays map[string]int) []string {
	sorted := append([]string(nil), all...)
	sort.SliceStable(sorted, func(i, j int) bool {
		di, dj := delays[sorted[i]], delays[sorted[j]]
		if di == 0 && dj == 0 {
			return false
		}
		if di == 0 {
			return false
		}
		if dj == 0 {
			return true
		}
		return di < dj
	})
	return sorted
}

func renderProxyNodeList(w io.Writer, sorted []string, current string, delays map[string]int, mode nodeListMode) {
	displayed := sorted
	if mode == nodeListPreview && len(displayed) > nodeListPreviewLimit {
		displayed = displayed[:nodeListPreviewLimit]
	}
	switch mode {
	case nodeListVerbose:
		// ll：单列详细表格（# / 当前 / 延迟 / 节点），便于一眼看清每个节点状态
		fmt.Fprintf(w, "  %3s  %-4s  %-8s  %s\n", "#", "当前", "延迟", "节点")
		for i, n := range displayed {
			currentText := ""
			if n == current {
				currentText = "当前"
			}
			fmt.Fprintf(w, "  %3d  %-4s  %s  %s\n",
				i+1, currentText, delayLabel(delays[n]), n)
		}
	case nodeListCompact:
		// ls：Linux `ls` 风格多列网格（列优先：第一列装延迟最低的几个）
		renderCompactNodeGrid(w, displayed, current, delays)
	default:
		// preview：首屏前 20 个单列概览，用户一眼能看到编号→名字→延迟对齐
		for i, n := range displayed {
			mark := "  "
			if n == current {
				mark = "✓ "
			}
			fmt.Fprintf(w, "  %2d  %s%s  %s\n",
				i+1, mark, padRightWide(n, 30), delayLabel(delays[n]))
		}
	}
}

// renderCompactNodeGrid 按 Linux `ls` 风格把节点紧凑铺成多列网格。
//   - 列优先（column-major）：第一列装前 rows 个最快的，读起来跟 `ls` 一致；
//   - 单元格宽度 = 最宽一项 + 2 空格间距，按终端宽度算列数，CJK/emoji 算 2 列；
//   - 延迟带颜色时打印彩色串，但对齐按无色宽度来，不会被 ANSI 序列撑歪。
func renderCompactNodeGrid(w io.Writer, sorted []string, current string, delays map[string]int) {
	if len(sorted) == 0 {
		return
	}
	plain := make([]string, len(sorted))
	colored := make([]string, len(sorted))
	maxW := 0
	for i, n := range sorted {
		mark := " "
		if n == current {
			mark = "✓"
		}
		plain[i] = fmt.Sprintf("%2d%s %s  %s", i+1, mark, n, delayPlain(delays[n]))
		colored[i] = fmt.Sprintf("%2d%s %s  %s", i+1, mark, n, delayLabel(delays[n]))
		if dw := displayWidth(plain[i]); dw > maxW {
			maxW = dw
		}
	}
	const (
		gap    = 2 // 列与列之间的空格数
		indent = 2 // 整块左侧缩进
	)
	cellW := maxW + gap
	avail := terminalWidth() - indent
	cols := avail / cellW
	if cols < 1 {
		cols = 1
	}
	if cols > len(sorted) {
		cols = len(sorted)
	}
	rows := (len(sorted) + cols - 1) / cols
	for r := 0; r < rows; r++ {
		fmt.Fprint(w, "  ")
		for c := 0; c < cols; c++ {
			idx := c*rows + r
			if idx >= len(sorted) {
				break
			}
			fmt.Fprint(w, colored[idx])
			// 末列不补 padding，避免在行尾多出一段空白
			if c < cols-1 && c*rows+r+rows < len(sorted) {
				pad := cellW - displayWidth(plain[idx])
				if pad < 0 {
					pad = 0
				}
				fmt.Fprint(w, strings.Repeat(" ", pad))
			}
		}
		fmt.Fprintln(w)
	}
}

// delayPlain 是 delayLabel 的无色版，仅用于计算对齐宽度。
func delayPlain(ms int) string {
	if ms <= 0 {
		return "—"
	}
	return fmt.Sprintf("%4d ms", ms)
}

// terminalWidth 尽量拿到当前终端宽度。拿不到时退回 80 列的保守值。
// 这里不引入新依赖，优先读 COLUMNS 环境变量（大多数交互 shell 会设）。
func terminalWidth() int {
	if s := os.Getenv("COLUMNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 80
}
