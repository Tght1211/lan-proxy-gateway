//go:build !linux && !darwin

package firewall

type unsupportedManager struct{}

func newPlatformManager() Manager { return unsupportedManager{} }

func (unsupportedManager) Apply(Config) (Report, error) { return Report{}, ErrNotSupported }
func (unsupportedManager) Remove() error                { return nil }

// DisablePF is a no-op here (pf only exists on macOS).
func DisablePF() error { return nil }
