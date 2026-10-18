package vpn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	bpc "github.com/wwricu/bad-proxy-core/core"
)

// Mirrors the desktop GUI's latency probe (engine.go dialThroughProxy /
// app.go TestNode): a real HTTP request through a local core inbound, not a
// bare TCP dial, so "fast" always means the whole node path carries traffic.

const testTimeout = 10 * time.Second

// probeURL is overridable in tests; the desktop app uses the same gstatic 204 endpoint.
var probeURL = "http://www.gstatic.com/generate_204"

// TestNode probes one node without touching the running tunnel: it starts a
// throwaway engine on a random high SOCKS port with that node as the only
// outbound, dials the 204 probe through it, and stops it. cfgJSON is the same
// Config document as Start; its routing mode is ignored (global is forced).
func TestNode(cfgJSON string) (string, error) {
	var cfg Config
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		return "", fmt.Errorf("bad config: %w", err)
	}
	cfg.Routing = "global"
	coreCfg, err := buildCoreConfig(cfg, 0)
	if err != nil {
		return "", err
	}
	for attempt := 0; attempt < 3; attempt++ {
		port, err := freePort()
		if err != nil {
			continue
		}
		c := coreCfg
		c.Inbounds[0].Port = strconv.Itoa(port)
		np, err := bpc.New(c)
		if err != nil {
			continue
		}
		np.Start()
		defer np.Stop()
		addr := "127.0.0.1:" + strconv.Itoa(port)
		// core binds inside a goroutine, so the inbound is not necessarily
		// up when Start returns; probing too early looks exactly like a dead
		// node. Wait for the port to accept before dialing through it.
		if err := waitListening(addr, 3*time.Second); err != nil {
			continue
		}
		return probe(addr)
	}
	return "", errors.New("no free test port")
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitListening(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			c.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// probe makes one request through the SOCKS5 inbound at socksAddr and returns
// the round-trip latency rounded like the desktop app (to the ms).
func probe(socksAddr string) (string, error) {
	client := &http.Client{
		Timeout: testTimeout,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				p, err := strconv.Atoi(port)
				if err != nil {
					return nil, err
				}
				// The domain is handed to core as ATYP=3, so router domain
				// rules decide the outbound even here.
				return socksDial(socksAddr, host, uint16(p))
			},
		},
	}
	start := time.Now()
	resp, err := client.Get(probeURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	elapsed := time.Since(start)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return elapsed.Round(time.Millisecond).String(), fmt.Errorf("proxy returned %s", resp.Status)
	}
	return elapsed.Round(time.Millisecond).String(), nil
}
