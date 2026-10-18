package vpn

import (
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

// socksStub is a no-auth SOCKS5 server. Without pipe it only records the
// CONNECT target and replies success (handshake assertions stay offline);
// with pipe it also dials the target and copies both ways (end-to-end probes).
func socksStub(t *testing.T, pipe bool) (addr string, targets chan string) {
	t.Helper()
	tgt := make(chan string, 16)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				hello := make([]byte, 2)
				if _, err := io.ReadFull(c, hello); err != nil || hello[0] != 5 || hello[1] < 1 {
					return
				}
				methods := make([]byte, hello[1])
				if _, err := io.ReadFull(c, methods); err != nil {
					return
				}
				if _, err := c.Write([]byte{5, 0}); err != nil {
					return
				}
				head := make([]byte, 4)
				if _, err := io.ReadFull(c, head); err != nil || head[0] != 5 || head[1] != 1 {
					return
				}
				var host string
				switch head[3] {
				case 1:
					ip := make([]byte, 4)
					if _, err := io.ReadFull(c, ip); err != nil {
						return
					}
					host = net.IP(ip).String()
				case 3:
					var l [1]byte
					if _, err := io.ReadFull(c, l[:]); err != nil {
						return
					}
					name := make([]byte, l[0])
					if _, err := io.ReadFull(c, name); err != nil {
						return
					}
					host = string(name)
				case 4:
					ip := make([]byte, 16)
					if _, err := io.ReadFull(c, ip); err != nil {
						return
					}
					host = net.IP(ip).String()
				default:
					return
				}
				var pb [2]byte
				if _, err := io.ReadFull(c, pb[:]); err != nil {
					return
				}
				target := net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(pb[:]))))
				tgt <- target
				if !pipe {
					c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
					return
				}
				up, err := net.DialTimeout("tcp", target, 5*time.Second)
				if err != nil {
					c.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
					return
				}
				defer up.Close()
				c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
				pump(c, up)
			}()
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String(), tgt
}

func TestSocksDialSendsDomainAsATYP3(t *testing.T) {
	addr, targets := socksStub(t, false)
	c, err := socksDial(addr, "Example.COM", 443)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	select {
	case got := <-targets:
		if got != "Example.COM:443" {
			t.Fatalf("stub saw %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stub never saw the connect request")
	}
}

func TestSocksDialSendsIPv4AsATYP1(t *testing.T) {
	addr, targets := socksStub(t, false)
	c, err := socksDial(addr, "203.0.113.7", 8080)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	if got := <-targets; got != "203.0.113.7:8080" {
		t.Fatalf("stub saw %q", got)
	}
}

func TestSocksDialRejectsOverlongName(t *testing.T) {
	addr, _ := socksStub(t, false)
	long := make([]byte, 256)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := socksDial(addr, string(long), 80); err == nil {
		t.Fatal("expected rejection of >255-byte target name")
	}
}
