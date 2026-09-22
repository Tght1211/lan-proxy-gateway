package relay

import (
	"context"
	"net"
	"sync"
	"time"
)

// boundHandshake applies both the dialer's timeout and caller cancellation to
// protocol negotiation. Finish detaches cancellation before returning a live
// tunnel, so canceling a completed dial cannot kill that tunnel later.
func boundHandshake(ctx context.Context, conn net.Conn, timeout time.Duration) func() error {
	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()); close(done) })
	var once sync.Once
	var err error
	return func() error {
		once.Do(func() {
			if !stop() {
				<-done
			}
			err = ctx.Err()
			_ = conn.SetDeadline(time.Time{})
		})
		return err
	}
}
