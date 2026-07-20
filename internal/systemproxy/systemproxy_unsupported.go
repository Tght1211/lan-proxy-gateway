//go:build !darwin

package systemproxy

func enable(Mode, string, int) error { return ErrNotSupported }
func disable() error                 { return ErrNotSupported }
func get() (Status, error)           { return Status{}, ErrNotSupported }
