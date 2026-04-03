//go:build windows

package cmd

import (
	"os"

	"golang.org/x/sys/windows"
)

func init() {
	// Set console to UTF-8 (code page 65001) to prevent Chinese garbled output.
	// Windows cmd.exe defaults to GBK (936) which mangles Go's UTF-8 output.
	windows.SetConsoleOutputCP(65001)
	windows.SetConsoleCP(65001)

	// Enable ANSI virtual terminal processing so color escape codes render correctly
	// on Windows 10+ terminals instead of appearing as raw escape characters.
	for _, f := range []*os.File{os.Stdout, os.Stderr} {
		h := windows.Handle(f.Fd())
		var mode uint32
		if windows.GetConsoleMode(h, &mode) == nil {
			windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
		}
	}
}
