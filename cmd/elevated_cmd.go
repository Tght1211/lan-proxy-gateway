package cmd

// elevatedCmd returns a shell-ready sudo invocation for `gateway <sub>`.
// Pass sub="" to get the bare invocation (no subcommand).
func elevatedCmd(sub string) string {
	name := "gateway"
	if sub != "" {
		name = name + " " + sub
	}
	return "sudo " + name
}
