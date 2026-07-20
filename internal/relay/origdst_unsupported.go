//go:build !linux && !darwin

package relay

// NewPlatformOrigDST returns ErrOrigDSTNotSupported on platforms without a
// transparent original-destination implementation.
func NewPlatformOrigDST() (OrigDSTResolver, error) {
	return nil, ErrOrigDSTNotSupported
}
