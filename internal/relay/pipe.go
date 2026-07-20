package relay

import (
	"io"
	"net"
	"sync"
	"time"
)

// pipe relays bytes between a (client) and b (upstream) in both directions,
// accounting them to the tracked connection. Half-closes are propagated so
// protocols that signal end-of-request via FIN work correctly.
func pipe(a, b net.Conn, tc *TrackedConn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		copyOne(b, a, tc.AddUp) // client → upstream = upload
	}()
	go func() {
		defer wg.Done()
		copyOne(a, b, tc.AddDown) // upstream → client = download
	}()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Minute):
		// one direction stalled after the other finished; force-close both
		a.Close()
		b.Close()
		<-done
	}
}

func copyOne(dst, src net.Conn, count func(int64)) {
	_, _ = io.Copy(countingWriter{w: dst, count: count}, src)
	// propagate EOF to the far side as a half-close when possible
	if tc, ok := dst.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	} else {
		_ = dst.Close()
	}
}

type countingWriter struct {
	w     io.Writer
	count func(int64)
}

func (cw countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.count(int64(n))
	return n, err
}
