package agentskill

import (
	"archive/zip"
	"bytes"
	"io"
	"os/exec"
	"testing"
)

func TestInspectHelper(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python 3 is required to validate the optional read-only helper")
	}
	output, err := exec.Command(python, "-B", "inspect_test.py").CombinedOutput()
	if err != nil {
		t.Fatalf("inspect helper tests failed: %v\n%s", err, output)
	}
}

func TestExportIsSelfContainedStaticSkill(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteZip(&buf); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{
		"lan-proxy-gateway/SKILL.md":                   "content/lan-proxy-gateway/SKILL.md",
		"lan-proxy-gateway/references/commands.md":     "content/lan-proxy-gateway/references/commands.md",
		"lan-proxy-gateway/scripts/gateway_inspect.py": "content/lan-proxy-gateway/scripts/gateway_inspect.py",
	}
	if len(z.File) != len(expected) {
		t.Fatalf("unexpected archive files: %d", len(z.File))
	}
	for _, f := range z.File {
		source, ok := expected[f.Name]
		if !ok {
			t.Fatalf("unexpected archive entry: %s", f.Name)
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		want, err := content.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("export differs from static source: %s", f.Name)
		}
	}
}
