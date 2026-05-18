//go:build windows

package cmd

import "errors"

// reexecForUpdateInstall 在 Windows 上不会被调用：runUpdate 已先校验 admin 并
// 在非 admin 时直接返回错误。这里保留一个返回错误的实现以满足平台编译。
func reexecForUpdateInstall(tag, tmpPath string) error {
	return errors.New("internal error: reexecForUpdateInstall should not be called on windows")
}
