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
	// Count each successful write so live streams appear in throughput and
	// bytes are assigned to the day they were transferred, not the close date.
	_, _ = io.Copy(trafficWriter{Conn: dst, count: count}, src)
	// propagate EOF to the far side as a half-close when possible
	if tc, ok := dst.(interface{ CloseWrite() error }); ok {
		_ = tc.CloseWrite()
	}
}

type trafficWriter struct {
	net.Conn
	count func(int64)
}

func (w trafficWriter) Write(p []byte) (int, error) {
	n, err := w.Conn.Write(p)
	if n > 0 {
		w.count(int64(n))
	}
	return n, err
}
