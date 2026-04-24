package server

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServerBindsToLoopbackOnly(t *testing.T) {
	s := New(nil, nil, "127.0.0.1:0") // port 0 = pick any free port
	go func() { _ = s.ListenAndServe() }()
	t.Cleanup(func() { _ = s.Shutdown() })

	// Give it a moment to bind.
	time.Sleep(50 * time.Millisecond)

	addr := s.Addr()
	if addr == "" {
		t.Fatal("Addr() returned empty after ListenAndServe")
	}

	// Confirm the listener is on loopback.
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1" {
		t.Errorf("bound to %q; must be 127.0.0.1", host)
	}

	// Confirm /api/health responds 200.
	resp, err := http.Get("http://" + addr + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/health: status %d", resp.StatusCode)
	}
}
