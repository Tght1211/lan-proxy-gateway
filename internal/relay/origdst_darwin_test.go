//go:build darwin

package relay

import (
	"testing"
	"unsafe"
)

// The DIOCNATLOOK ABI depends on pfiocNatlook matching the kernel's 84-byte
// layout; the ioctl number encodes that size. Guard both against drift.
func TestNatlookLayout(t *testing.T) {
	if size := unsafe.Sizeof(pfiocNatlook{}); size != 84 {
		t.Fatalf("sizeof(pfiocNatlook) = %d, want 84", size)
	}
	if dioCNATLOOK != 0xC0544417 {
		t.Fatalf("DIOCNATLOOK = %#x, want 0xC0544417", dioCNATLOOK)
	}
}
