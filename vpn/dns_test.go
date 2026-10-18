package vpn

import (
	"encoding/binary"
	"net"
	"strconv"
	"testing"
)

func buildQuery(t *testing.T, name string, qtype uint16) []byte {
	t.Helper()
	var b []byte
	head := make([]byte, 12)
	binary.BigEndian.PutUint16(head[0:2], 0xbeef)
	binary.BigEndian.PutUint16(head[4:6], 1)
	b = append(b, head...)
	for _, label := range splitLabels(name) {
		b = append(b, byte(len(label)))
		b = append(b, label...)
	}
	b = append(b, 0)
	var qt [4]byte
	binary.BigEndian.PutUint16(qt[0:2], qtype)
	binary.BigEndian.PutUint16(qt[2:4], 1)
	return append(b, qt[:]...)
}

func splitLabels(name string) []string {
	var out []string
	start := 0
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			out = append(out, name[start:i])
			start = i + 1
		}
	}
	if start < len(name) {
		out = append(out, name[start:])
	}
	return out
}

func TestDnsAnswerAAssignsFakeIP(t *testing.T) {
	pool := newFakePool()
	ans := dnsAnswer(buildQuery(t, "www.Example.com", 1), pool)
	if ans == nil {
		t.Fatal("no answer")
	}
	if qd, an := binary.BigEndian.Uint16(ans[4:6]), binary.BigEndian.Uint16(ans[6:8]); qd != 1 || an != 1 {
		t.Fatalf("qdcount=%d ancount=%d", qd, an)
	}
	if rcode := ans[3] & 0x0f; rcode != 0 {
		t.Fatalf("rcode=%d", rcode)
	}
	ip := net.IPv4(ans[len(ans)-4], ans[len(ans)-3], ans[len(ans)-2], ans[len(ans)-1]).To4()
	if ip[0] != 198 || ip[1] != 18 {
		t.Fatalf("fake ip %s outside 198.18.0.0/16", ip)
	}
	name, ok := pool.lookup(ip.To4())
	if !ok || name != "www.example.com" {
		t.Fatalf("pool maps %s -> %q,%v", ip, name, ok)
	}
	// Same name, different case: must reuse the address (rules match one domain).
	ans2 := dnsAnswer(buildQuery(t, "WWW.example.COM.", 1), pool)
	ip2 := net.IP(ans2[len(ans2)-4 : len(ans2)])
	if !ip2.Equal(ip) {
		t.Fatalf("case/trace mismatch: %s vs %s", ip, ip2)
	}
}

func TestDnsAnswerAAAAEmptyNoError(t *testing.T) {
	pool := newFakePool()
	ans := dnsAnswer(buildQuery(t, "example.com", 28), pool)
	if ans == nil {
		t.Fatal("no answer")
	}
	if an := binary.BigEndian.Uint16(ans[6:8]); an != 0 {
		t.Fatalf("AAAA ancount=%d, want 0", an)
	}
	if rcode := ans[3] & 0x0f; rcode != 0 {
		t.Fatalf("rcode=%d", rcode)
	}
}

func TestDnsAnswerOtherTypeRefused(t *testing.T) {
	pool := newFakePool()
	ans := dnsAnswer(buildQuery(t, "example.com", 15), pool) // MX
	if ans == nil {
		t.Fatal("no answer")
	}
	if rcode := ans[3] & 0x0f; rcode != 5 {
		t.Fatalf("rcode=%d, want REFUSED", rcode)
	}
}

func TestDnsAnswerMalformed(t *testing.T) {
	pool := newFakePool()
	for _, bad := range [][]byte{nil, {}, {1, 2, 3}, make([]byte, 12), {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 'a'}} {
		if ans := dnsAnswer(bad, pool); ans != nil {
			t.Fatalf("answered a malformed query: %v", bad)
		}
	}
}

func TestFakePoolSkipsNetworkAndBroadcast(t *testing.T) {
	pool := newFakePool()
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		name := "h" + string(rune('a'+i%26)) + strconv.Itoa(i)
		ip := pool.assign(name).To4()
		last := ip[3]
		if last == 0 || last == 255 {
			t.Fatalf("%s assigned a .0/.255 address", name)
		}
		if seen[ip.String()] {
			t.Fatalf("duplicate address %s for %s", ip, name)
		}
		seen[ip.String()] = true
	}
}
