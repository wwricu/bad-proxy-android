Android client for bad-proxy-core: a `VpnService` + tun2socks front end that runs
the engine inside the app, so no root and no second process.

## Install

Android 7.0 (API 24) or newer, on `arm64-v8a` or `x86_64` — the APK carries no
32-bit ARM build. Open the node editor, fill in server / port / UUID / transport
by hand (there is no share-link import yet), select the node, press Connect and
accept the system VPN dialog.

## This build

* Node list with a per-node latency test (long-press a node → Test)
* Global, mainland-whitelist and GFW-blacklist routing modes
* Auto-connect on boot
* Ongoing notification with a Disconnect action

## Known limitations

* TCP only: core has no UDP relay, so QUIC traffic falls back to TCP.
* IPv6 is not routed; a v6-only path bypasses the tunnel.
* Apps that resolve for themselves (browser DoH/DoQ) hand us a bare IP, which
  matches no domain rule and takes the mode's default outbound.
