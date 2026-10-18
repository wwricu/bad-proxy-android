package vpn

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wwricu/bad-proxy-core/router"
)

// rules.dat ships in the APK assets and is unpacked to filesDir at runtime;
// the split modes' decisions are only as good as the dataset wired into
// buildCoreConfig, so exercise the real one. It is downloaded by build.sh
// rather than committed, so a checkout that never built cannot run this.
func realRules(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "android", "app", "src", "main", "assets", "rules.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skip("rules.dat missing, run ./build.sh first:", err)
	}
	return p
}

func TestCNWhitelistDecisions(t *testing.T) {
	r, err := router.NewRouter("direct", []string{"cn"}, realRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || !r.MatchAny("www.baidu.com") {
		t.Error("www.baidu.com must match the cn route (direct)")
	}
	if r == nil || r.MatchAny("www.google.com") {
		t.Error("www.google.com must not match the cn route")
	}
}

func TestGFWBlacklistDecisions(t *testing.T) {
	r, err := router.NewRouter("proxy", []string{"geolocation-!cn"}, realRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || !r.MatchAny("www.google.com") {
		t.Error("www.google.com must match geolocation-!cn (proxy)")
	}
	if r == nil || r.MatchAny("www.baidu.com") {
		t.Error("www.baidu.com must not match geolocation-!cn")
	}
}
