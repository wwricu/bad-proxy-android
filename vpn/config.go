// Package vpn exposes the embedded bad-proxy engine to Android through
// gomobile. Start() takes the tun fd handed over by VpnService plus a JSON
// config, runs a local SOCKS5 inbound (core) and a gvisor netstack that turns
// tun packets into SOCKS5 connections.
package vpn

// Config is the JSON document the Kotlin layer passes to Start. It mirrors
// the desktop GUI's state file but only the parts that make sense on a
// device that has no user-facing listen ports.
type Config struct {
	Nodes     []Node `json:"nodes"`
	Active    int    `json:"active"`
	Routing   string `json:"routing_mode"` // global | cn-whitelist | gfw-blacklist
	RulesPath string `json:"rules_path"`   // on-disk rules.dat for named lists
	// LogPath is where the engine writes its own diagnostics. The app must
	// pass a path it can actually write; on Android TMPDIR is not writable.
	LogPath string `json:"log_path"`
}

type Node struct {
	Name      string `json:"name"`
	Protocol  string `json:"protocol"` // btp, vless
	Server    string `json:"server"`
	Port      string `json:"port"`
	Secret    string `json:"secret"`
	Transport string `json:"transport"`
	Path      string `json:"path"`
	Authority string `json:"authority"`
	Padding   string `json:"padding"`
}
