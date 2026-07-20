package relay

import (
	"io"
	"net"
	"time"
)

const pipeDrainTimeout = 2 * time.Minute

// pipe relays bytes between a (client) and b (upstream) in both directions,
// accounting them to the tracked connection. Half-closes are propagated so
// protocols that signal end-of-request via FIN work correctly.
func pipe(a, b net.Conn, tc *TrackedConn) {
	pipeWithDrainTimeout(a, b, tc, pipeDrainTimeout)
}

func pipeWithDrainTimeout(a, b net.Conn, tc *TrackedConn, drainTimeout time.Duration) {
	done := make(chan struct{}, 2)
	go func() {
		defer func() { done <- struct{}{} }()
		copyOne(b, a, tc.AddUp) // client → upstream = upload
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		copyOne(a, b, tc.AddDown) // upstream → client = download
	}()

	// A healthy long-lived stream may transfer in both directions for hours.
	// Start the drain timeout only after one direction reaches EOF.
	<-done
	timer := time.NewTimer(drainTimeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		// one direction stalled after the other finished; force-close both
		a.Close()
		b.Close()
		<-done
	}
}

func copyOne(dst, src net.Conn, count func(int64)) {
	// Keep the concrete net.Conn values visible to io.Copy. In particular this
	// preserves TCP splice on Linux instead of forcing every packet through a
	// userspace countingWriter. Totals become visible when the direction ends.
	n, _ := io.Copy(dst, src)
	count(n)
	// propagate EOF to the far side as a half-close when possible
	if tc, ok := dst.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	} else {
		_ = dst.Close()
	}
}
