# bad-proxy-android

Android client for [bad-proxy-core](https://github.com/wwricu/bad-proxy-core):
the engine runs in-process and the whole device is routed through it with the
system VPN API (`VpnService`), so no root and no proxy settings anywhere else.

## How it works

```
apps -> tun0 (VpnService) -> gvisor netstack (tun2socks) -> SOCKS5 127.0.0.1
     -> bad-proxy-core (vless/btp over tls/xhttp) -> node
```

* `vpn/` is the Go engine, bound into `android/app/libs/vpn.aar` by `gomobile`.
* DNS on the tunnel is answered with fake addresses (198.18.0.0/16) and mapped
  back to the domain before the SOCKS request, so core's domain rules keep
  working even though the app never sees the real IP.
* The engine's own uid is excluded from the tunnel
  (`addDisallowedApplication`). Without that, core's resolver queries and direct
  dials would be answered by its own fake DNS and every connection would loop
  back into the tunnel.

## Build

    ./build.sh

Produces `android/app/build/outputs/apk/debug/app-debug.apk`. See the header of
`build.sh` for the toolchain it expects, and point
`android/gradle.properties:org.gradle.java.home` at your own JDK (it is the
Android Studio bundle on the machine this was developed on).
`go.mod` replaces core with `../bad-proxy-core`, so keep the two checkouts side
by side.

## Routing modes

Only the routing mode and the node list are configurable; the desktop app's
listen ports and inbound settings do not exist here.

| Mode | Decision |
| --- | --- |
| Global | everything through the node |
| CN whitelist | mainland domains direct, the rest through the node |
| GFW blacklist | non-mainland domains through the node, the rest direct |

The lists come from `rules.dat` (a build of
[domain-list-community](https://github.com/wwricu/domain-list-community)),
unpacked from assets on first run.

## Limitations

* **TCP only.** core has no UDP relay, so QUIC/UDP traffic is dropped and
  clients fall back to TCP; DNS is answered locally.
* **IPv6 is not routed** (only a `0.0.0.0/0` route is added), so v6-capable
  apps on v6-only paths bypass the tunnel.
* Clients that resolve by themselves (browser DoH/DoQ) hand us a bare IP, which
  matches no domain rule and therefore takes the mode's default outbound.
* On the Android emulator, connections that *bypass* the tunnel (the direct
  half of the two split modes) do not complete: the SYN leaves on the physical
  interface and the reply is dropped by the emulator's ingress path. The
  routing decision itself is verifiable there, the direct transfer is not.
