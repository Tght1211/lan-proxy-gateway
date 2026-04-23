package console

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestSortProxyNodesByDelay(t *testing.T) {
	got := sortProxyNodes([]string{"timeout", "fast", "medium"}, map[string]int{
		"fast":    80,
		"medium":  320,
		"timeout": 0,
	})
	want := []string{"fast", "medium", "timeout"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sortProxyNodes() = %v, want %v", got, want)
	}
}

func TestRenderProxyNodeList_PreviewAndFullModes(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	nodes := make([]string, 0, nodeListPreviewLimit+2)
	delays := make(map[string]int, nodeListPreviewLimit+2)
	for i := 1; i <= nodeListPreviewLimit+2; i++ {
		name := fmt.Sprintf("node-%02d", i)
		nodes = append(nodes, name)
		delays[name] = i * 10
	}

	var preview bytes.Buffer
	renderProxyNodeList(&preview, nodes, "node-01", delays, nodeListPreview)
	if strings.Contains(preview.String(), "node-21") || strings.Contains(preview.String(), "node-22") {
		t.Fatalf("preview mode should hide nodes past %d:\n%s", nodeListPreviewLimit, preview.String())
	}

	var compact bytes.Buffer
	renderProxyNodeList(&compact, nodes, "node-01", delays, nodeListCompact)
	if !strings.Contains(compact.String(), "node-21") || !strings.Contains(compact.String(), "node-22") {
		t.Fatalf("compact mode should include full list:\n%s", compact.String())
	}
	// Linux `ls` 风格：同一行应该能塞进多个节点（多列），不是一行一个
	firstLine := strings.SplitN(compact.String(), "\n", 2)[0]
	if strings.Count(firstLine, "node-") < 2 {
		t.Fatalf("compact mode should render multi-column grid, but first line has fewer than 2 nodes:\n%q", firstLine)
	}

	var verbose bytes.Buffer
	renderProxyNodeList(&verbose, nodes[:2], "node-02", delays, nodeListVerbose)
	if !strings.Contains(verbose.String(), "#") || !strings.Contains(verbose.String(), "当前") {
		t.Fatalf("verbose mode missing header/current marker:\n%s", verbose.String())
	}
}

func TestRenderCompactGridColumnMajor(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	// 模拟窄终端：10 个节点、适中延迟；COLUMNS 够宽让布局至少 2 列
	t.Setenv("COLUMNS", "80")

	nodes := make([]string, 10)
	delays := map[string]int{}
	for i := 0; i < 10; i++ {
		n := fmt.Sprintf("n%d", i+1)
		nodes[i] = n
		delays[n] = (i + 1) * 10
	}

	var buf bytes.Buffer
	renderProxyNodeList(&buf, nodes, "n1", delays, nodeListCompact)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	// 列优先：最快的（前几个）应该落在第一列，而不是散在第一行
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), "1") {
		t.Fatalf("first cell should be idx 1 (fastest), got first line: %q", lines[0])
	}
	// 至少 2 行（列优先分布 10 项到几列）
	if len(lines) < 2 {
		t.Fatalf("expected multi-row layout, got %d lines:\n%s", len(lines), buf.String())
	}
}

func TestRenderCompactGridNarrowTermFallsBackToSingleCol(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	// 窄终端（COLUMNS=10）：每个 item 大于可用宽度 → 每行一项
	t.Setenv("COLUMNS", "10")

	nodes := []string{"really-long-node-A", "really-long-node-B"}
	delays := map[string]int{"really-long-node-A": 80, "really-long-node-B": 100}

	var buf bytes.Buffer
	renderProxyNodeList(&buf, nodes, "", delays, nodeListCompact)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("narrow terminal should fall back to single column (2 lines), got %d:\n%s", len(lines), buf.String())
	}
}
