package vpn

import (
	"testing"

	"github.com/wwricu/bad-proxy-core/router"
)

func testNode() Node {
	return Node{
		Name: "n", Protocol: "vless", Server: "host.example", Port: "443",
		Secret: "uuid", Transport: "xhttp", Path: "/x", Authority: "host.example",
	}
}

func cfgWith(routing string) Config {
	return Config{
		Nodes:     []Node{testNode()},
		Active:    0,
		Routing:   routing,
		RulesPath: "/tmp/rules.dat",
	}
}

func TestBuildCoreConfigGlobal(t *testing.T) {
	c, err := buildCoreConfig(cfgWith("global"), 20080)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Outbounds) != 1 || c.Outbounds[0].Tag != "" {
		t.Fatalf("global must be a single anonymous node outbound, got %+v", c.Outbounds)
	}
	if len(c.Router) != 0 {
		t.Fatalf("global must have no routes, got %+v", c.Router)
	}
	if !c.Outbounds[0].Transport.Tls {
		t.Fatal("tls must be forced on")
	}
	if c.Inbounds[0].Port != "20080" {
		t.Fatalf("socks port = %s", c.Inbounds[0].Port)
	}
}

func TestBuildCoreConfigCNWhitelist(t *testing.T) {
	c, err := buildCoreConfig(cfgWith("cn-whitelist"), 20080)
	if err != nil {
		t.Fatal(err)
	}
	// first outbound is the default (node), direct is the tagged second
	if c.Outbounds[0].Tag != "" || c.Outbounds[1].Tag != "direct" {
		t.Fatalf("outbound tags: %+v", c.Outbounds)
	}
	if len(c.Router) != 1 || c.Router[0].Tag != "direct" || !routerRule(c.Router[0], "cn") {
		t.Fatalf("routes: %+v", c.Router)
	}
}

func TestBuildCoreConfigGFWBlacklist(t *testing.T) {
	c, err := buildCoreConfig(cfgWith("gfw-blacklist"), 20080)
	if err != nil {
		t.Fatal(err)
	}
	if c.Outbounds[0].Tag != "" || c.Outbounds[1].Tag != "proxy" {
		t.Fatalf("outbound tags: %+v", c.Outbounds)
	}
	if len(c.Router) != 1 || c.Router[0].Tag != "proxy" || !routerRule(c.Router[0], "geolocation-!cn") {
		t.Fatalf("routes: %+v", c.Router)
	}
}

func routerRule(r router.Config, rule string) bool {
	for _, x := range r.Rules {
		if x == rule {
			return true
		}
	}
	return false
}

func TestBuildCoreConfigRejectsBadNodes(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"no nodes", Config{Active: 0}},
		{"active out of range", Config{Nodes: []Node{testNode()}, Active: 3}},
		{"missing server", Config{Nodes: []Node{{Port: "443", Secret: "x"}}, Active: 0}},
		{"bad port", Config{Nodes: []Node{{Server: "h", Port: "0", Secret: "x"}}, Active: 0}},
		{"huge port", Config{Nodes: []Node{{Server: "h", Port: "70000", Secret: "x"}}, Active: 0}},
	}
	for _, tc := range cases {
		if _, err := buildCoreConfig(tc.cfg, 20080); err == nil {
			t.Errorf("%s: expected error", tc.name)
		}
	}
}

func TestEncryptedTransportFoldsLegacyToTLS(t *testing.T) {
	for _, in := range []string{"", "tcp", "ws", "garbage", "tls", "TLS"} {
		if got := encryptedTransport(in); got != "tls" {
			t.Errorf("encryptedTransport(%q) = %q", in, got)
		}
	}
	if got := encryptedTransport("xhttp"); got != "xhttp" {
		t.Errorf("xhttp must stay xhttp, got %q", got)
	}
}
