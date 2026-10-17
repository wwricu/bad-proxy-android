package vpn

import (
	"encoding/binary"
	"net"
	"strings"
	"sync"
)

// fakePool hands out 198.18.0.0/16 addresses for queried domains. The apps
// then "connect to an IP", and the TCP forwarder maps that IP back to the
// domain so the core router can match its domain rules. Without this, DNS
// would resolve to real IPs locally and every domain-based rule would see a
// bare address and misroute.
type fakePool struct {
	mu     sync.Mutex
	byName map[string]net.IP
	byIP   map[string]string
	next   [4]byte
}

func newFakePool() *fakePool {
	return &fakePool{
		byName: map[string]net.IP{},
		byIP:   map[string]string{},
		next:   [4]byte{198, 18, 0, 0},
	}
}

func (p *fakePool) assign(name string) net.IP {
	p.mu.Lock()
	defer p.mu.Unlock()
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	if ip, ok := p.byName[name]; ok {
		return ip
	}
	for {
		p.next[3]++
		if p.next[3] == 0 {
			p.next[2]++
			if p.next[2] == 0 {
				p.next[1]++
			}
		}
		if p.next[1] > 19 { // wrap 198.18.255.x back to the start of the pool
			p.next = [4]byte{198, 18, 0, 0}
		}
		if p.next[3] != 0 && p.next[3] != 255 {
			break
		}
	}
	ip := net.IPv4(p.next[0], p.next[1], p.next[2], p.next[3])
	p.byName[name] = ip
	p.byIP[ip.String()] = name
	return ip
}

func (p *fakePool) lookup(ip net.IP) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	name, ok := p.byIP[ip.String()]
	return name, ok
}

// dnsAnswer answers A with a fake IP, AAAA with an empty NOERROR (so apps
// fall back to IPv4 instead of stalling), and other types with REFUSED.
// Only the first question of a well-formed query is understood, which is all
// stub resolvers send.
func dnsAnswer(query []byte, p *fakePool) []byte {
	end, name, qtype, ok := parseQuestion(query)
	if !ok {
		return nil
	}
	flags := uint16(0x8000|0x0080) | uint16(query[2]&0x01) // QR, RA, copy RD
	var answers []byte
	nanswers := 0
	switch qtype {
	case 1: // A
		ip := p.assign(name)
		answers = answerRecord(1, []byte{ip[12], ip[13], ip[14], ip[15]})
		nanswers = 1
	case 28: // AAAA: empty NOERROR
	default:
		flags |= 5 // REFUSED
	}
	out := make([]byte, 0, end+len(answers))
	head := make([]byte, 12)
	binary.BigEndian.PutUint16(head[0:2], binary.BigEndian.Uint16(query[0:2]))
	binary.BigEndian.PutUint16(head[2:4], flags)
	binary.BigEndian.PutUint16(head[4:6], 1)
	binary.BigEndian.PutUint16(head[6:8], uint16(nanswers))
	out = append(out, head...)
	out = append(out, query[12:end]...)
	out = append(out, answers...)
	return out
}

func parseQuestion(query []byte) (end int, name string, qtype uint16, ok bool) {
	if len(query) < 13 || binary.BigEndian.Uint16(query[4:6]) == 0 {
		return 0, "", 0, false
	}
	i := 12
	var labels []string
	for {
		if i >= len(query) {
			return 0, "", 0, false
		}
		l := int(query[i])
		if l == 0 {
			i++
			break
		}
		if l&0xC0 != 0 { // compression pointers do not appear in questions
			return 0, "", 0, false
		}
		if i+1+l > len(query) {
			return 0, "", 0, false
		}
		labels = append(labels, string(query[i+1:i+1+l]))
		i += 1 + l
	}
	if i+4 > len(query) {
		return 0, "", 0, false
	}
	qtype = binary.BigEndian.Uint16(query[i : i+2])
	return i + 4, strings.ToLower(strings.Join(labels, ".")), qtype, true
}

// answerRecord emits one RR with the name compressed as a pointer to the
// question name at offset 12, class IN, ttl 60.
func answerRecord(rtype uint16, rdata []byte) []byte {
	b := make([]byte, 10, 12+len(rdata))
	binary.BigEndian.PutUint16(b[0:2], 0xC00C)
	binary.BigEndian.PutUint16(b[2:4], rtype)
	binary.BigEndian.PutUint16(b[4:6], 1)
	binary.BigEndian.PutUint32(b[6:10], 60)
	var rl [2]byte
	binary.BigEndian.PutUint16(rl[:], uint16(len(rdata)))
	b = append(b, rl[:]...)
	return append(b, rdata...)
}
