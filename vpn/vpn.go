package vpn

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	bpc "github.com/wwricu/bad-proxy-core/core"
	"github.com/wwricu/bad-proxy-core/logger"
	"github.com/wwricu/bad-proxy-core/router"
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

// logFile is the fallback engine log path when the caller does not supply
// one. See Config.LogPath: on Android only the Kotlin layer knows a directory
// the app may write to.
var logFile = filepath.Join(os.TempDir(), "bad-proxy.log")

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
// Loop avoidance relies on the app being added to the VPN's allowed
// applications (Kotlin side): traffic from this process never enters the tun,
// so core's outbound dials and the direct UDP relays leave the device for
// real without needing socket protect().
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
	coreCfg, err := buildCoreConfig(cfg)
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

// buildCoreConfig mirrors the desktop GUI's engine wiring, minus the http
// inbound and the listen-port settings: a VPN client has no user-facing
// ports. Routing modes decide which domains go through the node:
//   - global:        everything via the node.
//   - cn-whitelist:  mainland domains direct, the rest via the node.
//   - gfw-blacklist: mainland + private ranges direct, geolocation-!cn via
//     the node (the classic bypass split).
func buildCoreConfig(cfg Config) (bpc.Config, error) {
	if cfg.Active < 0 || cfg.Active >= len(cfg.Nodes) {
		return bpc.Config{}, errors.New("no active node")
	}
	logPath := cfg.LogPath
	if logPath == "" {
		logPath = logFile
	}
	node := cfg.Nodes[cfg.Active]
	host, port := node.Server, mustAtoi(node.Port)
	if host == "" || port <= 0 || port > 65535 {
		return bpc.Config{}, errors.New("active node has bad server/port")
	}
	var (
		outbounds []bpc.OutboundConfig
		routers   []router.Config
	)
	switch cfg.Routing {
	case "cn-whitelist":
		outbounds = []bpc.OutboundConfig{nodeOutbound(node, ""), {Tag: "direct", Protocol: bpc.FREEDOM}}
		routers = []router.Config{{Tag: "direct", Rules: []string{"cn"}}}
	case "gfw-blacklist":
		outbounds = []bpc.OutboundConfig{{Tag: "", Protocol: bpc.FREEDOM}, nodeOutbound(node, "proxy")}
		routers = []router.Config{{Tag: "proxy", Rules: []string{"geolocation-!cn"}}}
	default: // global
		outbounds = []bpc.OutboundConfig{nodeOutbound(node, "")}
	}
	return bpc.Config{
		General: bpc.GeneralConfig{
			ConsoleLevel: "warn",
			LogFile:      logPath,
			LogLevel:     "debug",
		},
		Inbounds: []bpc.InboundConfig{
			{Protocol: "socks", Host: "127.0.0.1", Port: strconv.Itoa(socksPort)},
		},
		Outbounds:  outbounds,
		Router:     routers,
		RouterPath: cfg.RulesPath,
	}, nil
}

func nodeOutbound(n Node, tag string) bpc.OutboundConfig {
	return bpc.OutboundConfig{
		Tag:      tag,
		Protocol: n.Protocol,
		Secret:   n.Secret,
		Host:     n.Server,
		Port:     n.Port,
		Transport: bpc.TransportConfig{
			Type:      encryptedTransport(n.Transport),
			Path:      n.Path,
			Authority: n.Authority,
			Tls:       true,
			Padding:   n.Padding,
		},
	}
}

// encryptedTransport folds legacy/blank transports onto tls: the client
// never makes a plaintext hop (same rule as the desktop app).
func encryptedTransport(transport string) string {
	if transport == "xhttp" {
		return "xhttp"
	}
	return "tls"
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
