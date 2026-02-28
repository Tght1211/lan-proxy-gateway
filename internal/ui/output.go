package ui

import (
	"fmt"

	"github.com/fatih/color"
)

var (
	red    = color.New(color.FgRed)
	green  = color.New(color.FgGreen)
	yellow = color.New(color.FgYellow)
	blue   = color.New(color.FgBlue)
	cyan   = color.New(color.FgCyan)
	bold   = color.New(color.Bold)
	dim    = color.New(color.Faint)
)

func ShowLogo() {
	cyan.Println(`  _                  ____                      `)
	cyan.Println(` | |    __ _ _ __   |  _ \ _ __ _____  ___   _ `)
	cyan.Println(" | |   / _` | '_ \\  | |_) | '__/ _ \\ \\/ / | | |")
	cyan.Println(` | |__| (_| | | | | |  __/| | | (_) >  <| |_| |`)
	cyan.Println(` |_____\__,_|_| |_| |_|   |_|  \___/_/\_\\__, |`)
	cyan.Println(`                    Gateway                |___/ `)
	fmt.Println()
}

func Info(format string, a ...interface{}) {
	green.Printf("[INFO] ")
	fmt.Printf(format+"\n", a...)
}

func Warn(format string, a ...interface{}) {
	yellow.Printf("[WARN] ")
	fmt.Printf(format+"\n", a...)
}

func Error(format string, a ...interface{}) {
	red.Printf("[ERROR] ")
	fmt.Printf(format+"\n", a...)
}

func Success(format string, a ...interface{}) {
	green.Print("[")
	bold.Print("OK")
	green.Print("] ")
	fmt.Printf(format+"\n", a...)
}

func Step(num, total int, msg string) {
	blue.Printf("[%d/%d] ", num, total)
	fmt.Println(msg)
}

func Separator() {
	dim.Println("─────────────────────────────────────────")
}

func FormatBytes(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
