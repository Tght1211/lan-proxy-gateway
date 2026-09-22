// Package agentskill bundles the portable external-agent skill with the CLI.
package agentskill

import (
	"archive/zip"
	"embed"
	"io"
	"io/fs"
	"strings"
)

//go:embed content
var content embed.FS

func WriteZip(w io.Writer) error {
	z := zip.NewWriter(w)
	err := fs.WalkDir(content, "content", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := content.ReadFile(path)
		if err != nil {
			return err
		}
		entry, err := z.Create(strings.TrimPrefix(path, "content/"))
		if err != nil {
			return err
		}
		_, err = entry.Write(data)
		return err
	})
	closeErr := z.Close()
	if err != nil {
		return err
	}
	return closeErr
}
