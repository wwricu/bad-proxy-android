//go:build android

package vpn

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	bpc "github.com/wwricu/bad-proxy-core/core"
	"github.com/wwricu/bad-proxy-core/logger"
	tunsocks "github.com/xjasonlyu/tun2socks/v2/core"
	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	gstack "gvisor.dev/gvisor/pkg/tcpip/stack"
)

// socksListen is the loopback port the embedded core serves; only the
// netstack in this same process talks to it, so it never needs to be
// configurable and apps cannot reach it directly (they reach the tun).
const socksListen = "127.0.0.1:20080"

const socksPort = 20080

const tunMTU = 1500

type session struct {
	proxy   *bpc.Proxy
	stack   *gstack.Stack
	tunFd   int
	stopped bool
	mu      sync.Mutex
}

var (
	mu      sync.Mutex
	current *session
	// lastErr is why the most recent Start failed or the live session died; the
	// UI shows it so a broken node is not silently a dead tunnel.
	lastErr string
)

// LastError returns the most recent engine failure, empty when the last
// connect was clean.
func LastError() string {
	mu.Lock()
	defer mu.Unlock()
	return lastErr
}

// Start brings up the whole tunnel: the embedded core proxy with a loopback
// SOCKS5 inbound, and a gvisor netstack reading the tun fd handed over by
// VpnService. cfgJSON is a Config document; tunFd must stay open until Stop.
//
// Loop avoidance relies on the engine's uid being excluded from the tunnel
// (Kotlin side, addDisallowedApplication): traffic from this process never
// enters the tun, so core's outbound dials and the direct UDP relays leave
// the device for real without needing socket protect().
func Start(cfgJSON string, tunFd int64) (err error) {
	mu.Lock()
	defer mu.Unlock()
	// Record the outcome for the UI before returning; mu is already held, so
	// this writes lastErr directly rather than through LastError().
	defer func() {
		if err != nil {
			lastErr = err.Error()
		} else {
			lastErr = ""
		}
	}()
	if current != nil {
		return errors.New("vpn already running")
	}
	var cfg Config
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		return fmt.Errorf("bad config: %w", err)
	}
	coreCfg, err := buildCoreConfig(cfg, socksPort)
	if err != nil {
		return err
	}
	p, err := bpc.New(coreCfg)
	if err != nil {
		return fmt.Errorf("engine: %w", err)
	}
	p.Start()

	ep, err := fdbased.New(&fdbased.Options{
		FDs:                   []int{int(tunFd)},
		MTU:                   tunMTU,
		EthernetHeader:        false,
		PacketDispatchMode:    fdbased.Readv,
		MaxSyscallHeaderBytes: 0,
	})
	if err != nil {
		p.Stop()
		return fmt.Errorf("tun endpoint: %w", err)
	}
	st, err := tunsocks.CreateStack(&tunsocks.Config{
		LinkEndpoint: ep,
		TransportHandler: &handler{
			socksAddr: socksListen,
			pool:      newFakePool(),
		},
	})
	if err != nil {
		ep.Close()
		_ = unix.Close(int(tunFd))
		p.Stop()
		return fmt.Errorf("netstack: %w", err)
	}
	current = &session{proxy: p, stack: st, tunFd: int(tunFd)}
	logger.Debugf("vpn: stack up on fd %d, socks %s", tunFd, socksListen)
	return nil
}

// Stop tears the tunnel down. Safe to call when nothing is running.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if current == nil {
		return
	}
	s := current
	current = nil
	s.mu.Lock()
	if !s.stopped {
		s.stopped = true
		s.stack.Close()
		s.stack.Wait()
		s.proxy.Stop()
		_ = unix.Close(s.tunFd)
	}
	s.mu.Unlock()
	lastErr = "" // mu is already held here
}

// Running reports whether a tunnel session is live (poll-able from the UI).
func Running() bool {
	mu.Lock()
	defer mu.Unlock()
	return current != nil
}

// TestConnection probes the live tunnel through its loopback SOCKS inbound.
func TestConnection() (string, error) {
	if !Running() {
		return "", errors.New("vpn not running")
	}
	return probe(socksListen)
}
