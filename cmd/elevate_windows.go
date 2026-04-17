//go:build windows

package cmd

import "errors"

func reexecWithSudo() error {
	// Windows has no sudo; maybeElevate prints a different message above and exits.
	return errors.New("not supported on windows")
}
