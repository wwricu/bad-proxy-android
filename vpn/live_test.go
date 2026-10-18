package vpn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestLiveNode is opt-in: BADPROXY_LIVE=1 with the desktop GUI's state.json
// present. It exercises the whole TestNode path against the user's real node.
func TestLiveNode(t *testing.T) {
	if os.Getenv("BADPROXY_LIVE") != "1" {
		t.Skip("live test")
	}
	p := filepath.Join(os.Getenv("APPDATA"), "bad-proxy-gui", "state.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Nodes  []Node `json:"nodes"`
		Active int    `json:"active"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	for i := range s.Nodes {
		cfg, _ := json.Marshal(Config{Nodes: s.Nodes, Active: i, Routing: "global"})
		lat, err := TestNode(string(cfg))
		t.Logf("node %d (%s): %s err=%v", i, s.Nodes[i].Name, lat, err)
		if err != nil {
			t.Errorf("node %d failed", i)
		}
	}
}
