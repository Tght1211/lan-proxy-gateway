package relay

import (
	"io"
	"net"
	"testing"
)

// Exercise the forwarding loop and live byte accounting over local streams.
// This measures relay overhead, not Internet bandwidth.
func BenchmarkCopyOne(b *testing.B) {
	payload := make([]byte, 1<<20)
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	for b.Loop() {
		producer, src := net.Pipe()
		dst, consumer := net.Pipe()
		tc := NewTracker().Open("192.0.2.1", "video.example", 443, true, "tcp")
		done := make(chan error, 2)
		go func() {
			_, err := producer.Write(payload)
			producer.Close()
			done <- err
		}()
		go func() {
			_, err := io.Copy(io.Discard, consumer)
			consumer.Close()
			done <- err
		}()
		if err := copyOne(dst, src, tc.AddDown); err != nil {
			b.Fatal(err)
		}
		dst.Close()
		src.Close()
		for range 2 {
			if err := <-done; err != nil {
				b.Fatal(err)
			}
		}
		if tc.Down() != int64(len(payload)) {
			b.Fatalf("forwarded %d bytes", tc.Down())
		}
		tc.Close()
	}
}
