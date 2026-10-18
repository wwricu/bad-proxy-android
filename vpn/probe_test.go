package vpn

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func withProbeURL(t *testing.T, url string) {
	t.Helper()
	old := probeURL
	probeURL = url
	t.Cleanup(func() { probeURL = old })
}

func TestProbeSuccessThroughSocks(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer origin.Close()
	addr, _ := socksStub(t, true)
	withProbeURL(t, origin.URL)
	lat, err := probe(addr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := time.ParseDuration(lat); err != nil {
		t.Fatalf("latency %q is not a duration string", lat)
	}
	if !strings.HasSuffix(lat, "ms") {
		t.Fatalf("latency %q not rounded to ms", lat)
	}
}

func TestProbeNon2xxIsFailure(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer origin.Close()
	addr, _ := socksStub(t, true)
	withProbeURL(t, origin.URL)
	_, err := probe(addr)
	if err == nil || !strings.Contains(err.Error(), "proxy returned") {
		t.Fatalf("err = %v", err)
	}
}

func TestProbeUnreachableProxy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	if _, err := probe(addr); err == nil {
		t.Fatal("expected dial error")
	}
}

func TestTestNodeInputValidation(t *testing.T) {
	if _, err := TestNode("nonsense"); err == nil {
		t.Fatal("expected bad config error")
	}
	if _, err := TestNode(`{"nodes":[],"active":0,"routing_mode":"global"}`); err == nil {
		t.Fatal("expected no-active-node error")
	}
}

func TestTestNodeDeadNodeReportsFailure(t *testing.T) {
	// 127.0.0.1:9 (discard) refuses the tls handshake; the throwaway engine
	// must start, probe, fail fast and stop without leaving state behind.
	cfg := `{"nodes":[{"name":"dead","protocol":"vless","server":"127.0.0.1","port":"1","secret":"x","transport":"tls"}],"active":0,"routing_mode":"global"}`
	withProbeURL(t, "http://127.0.0.1:1/")
	if _, err := TestNode(cfg); err == nil {
		t.Fatal("expected failure against a dead node")
	}
}
