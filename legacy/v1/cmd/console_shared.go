package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

type consoleAction int

const (
	consoleActionNone consoleAction = iota
	consoleActionExit
	consoleActionRestart
	consoleActionStop
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func newConsoleClient(cfg *config.Config) *mihomo.Client {
	apiURL := mihomo.FormatAPIURL("127.0.0.1", cfg.Runtime.Ports.API)
	return mihomo.NewClient(apiURL, cfg.Runtime.APISecret)
}

func renderSectionTitle(title string) string {
	return title
}

func noteLine(text string) string {
	return "[note] " + text
}

func successLine(text string) string {
	return "[ok] " + text
}

func errorLine(text string) string {
	return "[error] " + text
}

func consoleOnOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func consoleState(enabled bool, onText, offText string) string {
	if enabled {
		return onText
	}
	return offText
}

func fallbackText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func stripANSI(s string) string {
	return ansiEscapePattern.ReplaceAllString(s, "")
}

func plainText(s string) string {
	return stripANSI(s)
}

func outputBlock(text string) []string {
	text = strings.TrimSpace(stripANSI(text))
	if text == "" {
		return nil
	}

	raw := splitLines(text)
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimRight(plainText(line), " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isSeparatorLine(trimmed) {
			continue
		}
		if isConsoleSectionTitle(trimmed) {
			if len(out) > 0 {
				out = append(out, "")
			}
			out = append(out, renderSectionTitle(trimmed))
			continue
		}
		out = append(out, "  "+trimmed)
	}
	return out
}

func isSeparatorLine(s string) bool {
	return strings.Trim(s, "─- ") == ""
}

func isConsoleSectionTitle(s string) bool {
	if strings.Contains(s, ":") {
		return false
	}
	if strings.HasPrefix(s, "[") {
		return false
	}
	return len([]rune(s)) <= 24
}

func printSimpleDetail(title string, lines []string) {
	ui.ShowLogo()
	color.New(color.Bold).Println(title)
	fmt.Println()
	ui.Separator()
	fmt.Println()
	for _, line := range lines {
		plain := strings.TrimRight(line, " \t")
		if strings.TrimSpace(plain) == "" {
			fmt.Println()
			continue
		}
		fmt.Printf("  %s\n", plain)
	}
	fmt.Println()
}

func promptSimpleValue(reader *bufio.Reader, label, current string, allowClear bool) (string, bool) {
	prompt := label
	if strings.TrimSpace(current) != "" {
		prompt += "（当前: " + current + "）"
	}
	if allowClear {
		prompt += "，输入 - 清空，回车取消"
	} else {
		prompt += "，回车取消"
	}
	fmt.Print("  " + prompt + ": ")

	input, _ := reader.ReadString('\n')
	value := strings.TrimSpace(input)
	if value == "" {
		fmt.Println("  已取消。")
		fmt.Println()
		return "", false
	}
	if allowClear && value == "-" {
		return "", true
	}
	return value, true
}

func promptMenuChoice(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.ToLower(strings.TrimSpace(input))
}

func confirmMenuAction(reader *bufio.Reader, prompt string) bool {
	fmt.Printf("  %s [y/N]: ", prompt)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	fmt.Println()
	return answer == "y" || answer == "yes"
}

func rejectRemovedTUIFlag(cmd *cobra.Command) bool {
	if cmd == nil || !cmd.Flags().Changed("tui") {
		return false
	}

	enabled, err := cmd.Flags().GetBool("tui")
	if err != nil || !enabled {
		return false
	}

	ui.Error("TUI 已移除，请直接使用当前默认的菜单式 CLI 控制台。")
	return true
}

func captureOutput(fn func()) string {
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	r, w, err := os.Pipe()
	if err != nil {
		return ""
	}

	os.Stdout = w
	os.Stderr = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	_ = r.Close()

	return <-done
}

func showOutputScreen(reader *bufio.Reader, title string, fn func()) {
	lines := outputBlock(captureOutput(fn))
	if len(lines) == 0 {
		lines = []string{noteLine("没有可展示的输出。")}
	}

	clearInteractiveScreen()
	printSimpleDetail(title, lines)
	waitEnter(reader)
}
