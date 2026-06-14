package console

import "strings"

// truncate 把字符串按字节数裁到 n，溢出加 …。
// 英文路径/URL 场景下字节 ≈ 字符，不用处理宽字符。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return s[:n-1] + "…"
}

// padRight 把字符串右填空格到指定宽度（按字节，仅 ASCII 用）。
func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// displayWidth 按等宽终端显示列数算字符串宽度。
// CJK / 日韩 / 全角 / Emoji 一律 2 列，ASCII / 普通符号 1 列。
// fmt 的 %-Ns 按字节，中文一字 3 字节占 2 列，导致对齐错位，这个函数补正。
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case r < 0x80:
			w++
		case r >= 0x1100 && r <= 0x115F, // Hangul jamo
			r >= 0x2E80 && r <= 0x303E,   // CJK 符号 / 部首
			r >= 0x3041 && r <= 0x33FF,   // Hiragana / Katakana
			r >= 0x3400 && r <= 0x9FFF,   // CJK Unified Ideographs
			r >= 0xAC00 && r <= 0xD7A3,   // Hangul
			r >= 0xF900 && r <= 0xFAFF,   // CJK compat
			r >= 0xFE30 && r <= 0xFE4F,   // CJK compat forms
			r >= 0xFF00 && r <= 0xFF60,   // 全角
			r >= 0xFFE0 && r <= 0xFFE6,   // 全角符号
			r >= 0x1F000 && r <= 0x1FFFF: // Emoji / supplementary
			w += 2
		default:
			w++
		}
	}
	return w
}

// padRightWide 按显示宽度右填空格。
func padRightWide(s string, cols int) string {
	need := cols - displayWidth(s)
	if need <= 0 {
		return s
	}
	return s + strings.Repeat(" ", need)
}

// --- util ---

func onOff(v bool) string {
	if v {
		return "开"
	}
	return "关"
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}
