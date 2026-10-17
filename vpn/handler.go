package vpn

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/wwricu/bad-proxy-core/logger"
	"github.com/xjasonlyu/tun2socks/v2/core/adapter"
)

func deadline() time.Time { return time.Now().Add(30 * time.Second) }

// handler turns netstack connections into local SOCKS5 (TCP, through the
// core proxy) or direct UDP relays (DNS is answered locally with fake IPs).
type handler struct {
	socksAddr string
	pool      *fakePool
}

func (h *handler) HandleTCP(c adapter.TCPConn) {
	id := c.ID()
	dst := net.IP(id.LocalAddress.AsSlice())
	port := id.LocalPort
	go func() {
		defer c.Close()
		target := dst.String()
		if name, ok := h.pool.lookup(dst.To4()); ok {
			target = name
		}
		logger.Debugf("vpn: tcp %s:%d -> %s:%d", id.RemoteAddress, id.RemotePort, target, port)
		up, err := socksDial(h.socksAddr, target, port)
		if err != nil {
			logger.Debugf("vpn: tcp %s:%d dial failed: %v", target, port, err)
			return
		}
		defer up.Close()
		pump(c, up)
	}()
}

func (h *handler) HandleUDP(c adapter.UDPConn) {
	id := c.ID()
	dstIP := net.IP(id.LocalAddress.AsSlice())
	dstPort := id.LocalPort
	go func() {
		defer c.Close()
		if dstPort == 53 {
			serveDNS(c, h.pool)
			return
		}
		if isFakeIP(dstIP) {
			// QUIC to a fake address cannot be proxied: the core SOCKS
			// inbound has no UDP, and the fake address is not routable
			// anyway. Dropping it is what makes apps fall back to TCP.
			return
		}
		logger.Debugf("vpn: udp relay %s:%d", dstIP, dstPort)
		relayUDP(c, dstIP, dstPort)
	}()
}

// isFakeIP reports whether addr came from the fake pool, i.e. whether it is a
// placeholder for a domain rather than a real destination.
func isFakeIP(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && v4[0] == 198 && v4[1] == 18
}

// pump copies both directions and half-closes each side when the other is
// done, so FIN propagates like a real proxy would.
func pump(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go copyHalf(b, a, &wg)
	go copyHalf(a, b, &wg)
	wg.Wait()
}

func copyHalf(dst, src net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	_, _ = io.Copy(dst, src)
	if cw, ok := dst.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	} else {
		_ = dst.Close()
	}
}

func serveDNS(c adapter.UDPConn, pool *fakePool) {
	buf := make([]byte, 2048)
	for {
		// A UDP session is a slot in the netstack's forwarder table, so every
		// loop below must be able to idle out or the table fills up and the
		// tunnel silently stops accepting new flows.
		if err := c.SetReadDeadline(deadline()); err != nil {
			return
		}
		n, src, err := c.ReadFrom(buf)
		if err != nil {
			return
		}
		ans := dnsAnswer(buf[:n], pool)
		if ans == nil {
			logger.Debugf("vpn: dns query unparseable (%d bytes)", n)
			continue
		}
		logger.Debugf("vpn: dns answered %d bytes", len(ans))
		if _, err := c.WriteTo(ans, src); err != nil {
			return
		}
	}
}

// udpIdle bounds how long a relayed UDP flow may sit unused before its
// session is released. The system resolver retries DNS-over-QUIC aggressively
// (thousands of flows in a couple of minutes on a quiet emulator), and each
// one holds a netstack session until it closes.
const udpIdle = 60 * time.Second

// relayUDP forwards non-DNS UDP straight out of the device. Our own process
// is kept off the tunnel by the system (the owner uid of a VPN never enters
// the tun it created), so these sockets leave via the real interface.
func relayUDP(c adapter.UDPConn, dstIP net.IP, dstPort uint16) {
	sock, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: dstIP, Port: int(dstPort)})
	if err != nil {
		logger.Debugf("vpn: udp relay %s:%d dial failed: %v", dstIP, dstPort, err)
		return
	}
	defer sock.Close()

	// Replies arrive asynchronously; a stop-and-wait relay here drops them and
	// makes QUIC retry, which is what turns background DNS noise into a flood.
	go func() {
		buf := make([]byte, 2048)
		for {
			if err := sock.SetReadDeadline(time.Now().Add(udpIdle)); err != nil {
				return
			}
			m, err := sock.Read(buf)
			if err != nil {
				return
			}
			if _, err := c.Write(buf[:m]); err != nil {
				return
			}
		}
	}()

	buf := make([]byte, 2048)
	for {
		if err := c.SetReadDeadline(time.Now().Add(udpIdle)); err != nil {
			return
		}
		n, _, err := c.ReadFrom(buf)
		if err != nil {
			return
		}
		if _, err := sock.Write(buf[:n]); err != nil {
			return
		}
	}
}
