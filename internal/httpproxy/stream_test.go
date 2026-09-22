package httpproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPStreamingFlushesBeforeOriginCompletes(t *testing.T) {
	finish := make(chan struct{})
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: ready\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-finish:
		case <-r.Context().Done():
		}
	}))
	defer origin.Close()
	defer close(finish)
	_, address := startProxy(t, Credentials{})
	client := proxyClient(t, address, "", "")
	client.Timeout = 500 * time.Millisecond
	resp, err := client.Get(origin.URL)
	if err != nil {
		t.Fatalf("stream headers/data buffered: %v", err)
	}
	defer resp.Body.Close()
	data := make([]byte, len("data: ready\n\n"))
	if _, err = io.ReadFull(resp.Body, data); err != nil {
		t.Fatal(err)
	}
	if string(data) != "data: ready\n\n" {
		t.Fatalf("wrong event %q", data)
	}
}
