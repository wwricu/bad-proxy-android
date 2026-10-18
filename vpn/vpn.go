package vpn

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"

	bpc "github.com/wwricu/bad-proxy-core/core"
	"github.com/wwricu/bad-proxy-core/router"
)

// logFile is the fallback engine log path when the caller does not supply
// one. See Config.LogPath: on Android only the Kotlin layer knows a directory
// the app may write to.
var logFile = filepath.Join(os.TempDir(), "bad-proxy.log")

// buildCoreConfig mirrors the desktop GUI's engine wiring, minus the http
// inbound and the listen-port settings: a VPN client has no user-facing
// ports. Routing modes decide which domains go through the node:
//   - global:        everything via the node.
//   - cn-whitelist:  mainland domains direct, the rest via the node.
//   - gfw-blacklist: mainland + private ranges direct, geolocation-!cn via
//     the node (the classic bypass split).
//
// socksPort selects the loopback inbound the netstack talks to: the tunnel's
// fixed 20080, or a random one per TestNode throwaway engine.
func buildCoreConfig(cfg Config, socksPort int) (bpc.Config, error) {
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
