package proxy

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
)

// ExtractProxies reads a Clash/mihomo YAML config file and extracts
// the "proxies:" section into a separate file for use as a proxy-provider.
// Returns the number of proxies extracted.
func ExtractProxies(inputPath, outputPath string) (int, error) {
	in, err := os.Open(inputPath)
	if err != nil {
		return 0, fmt.Errorf("无法打开配置文件: %w", err)
	}
	defer in.Close()

	var lines []string
	found := false
	count := 0

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "proxies:") {
			found = true
		} else if found && len(line) > 0 && unicode.IsLetter(rune(line[0])) {
			// Hit the next top-level key, stop
			break
		}

		if found {
			lines = append(lines, line)
			if strings.Contains(line, "- name:") {
				count++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("读取配置文件失败: %w", err)
	}

	if count == 0 {
		return 0, fmt.Errorf("未能从配置文件中提取到代理节点，请确认文件包含 proxies: 段落")
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return 0, fmt.Errorf("写入代理文件失败: %w", err)
	}

	return count, nil
}
