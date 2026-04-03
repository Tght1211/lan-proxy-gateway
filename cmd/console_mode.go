package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func resolveConsoleSimpleMode(cmd *cobra.Command, defaultSimple, simpleFlag, tuiFlag bool) (bool, error) {
	simpleChanged := cmd != nil && cmd.Flags().Changed("simple")
	tuiChanged := cmd != nil && cmd.Flags().Changed("tui")

	if simpleChanged && tuiChanged {
		if simpleFlag == tuiFlag {
			return false, fmt.Errorf("不能同时使用 --simple 和 --tui")
		}
		return simpleFlag, nil
	}
	if simpleChanged {
		return simpleFlag, nil
	}
	if tuiChanged {
		return !tuiFlag, nil
	}
	return defaultSimple, nil
}
