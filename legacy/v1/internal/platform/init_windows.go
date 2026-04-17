//go:build windows

package platform

import "runtime"

type impl struct{}

func New() Platform { return &impl{} }

func DetectArch() string {
	return runtime.GOARCH
}
