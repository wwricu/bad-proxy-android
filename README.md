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

    ./build.sh            # debug APK
    ./build.sh release    # release APK

Produces `android/app/build/outputs/apk/<type>/app-<type>.apk`. See the header
of `build.sh` for the toolchain it expects. `go.mod` replaces core with
`../bad-proxy-core`, so keep the two checkouts side by side (core's `dev`
branch, which is what CI pins — `master` still has the old package layout).

Gradle picks its JDK from `JAVA_HOME`, or from `org.gradle.java.home` in
`~/.gradle/gradle.properties` if you want a machine-local override (that is
where the Android Studio JBR path belongs — keeping it out of the repo is what
lets CI run).

## CI and releases

* `.github/workflows/ci.yml` runs on every push and PR: `go vet` + `go test`
  for the engine, then a real `gomobile` + Gradle build of the debug APK,
  uploaded as a workflow artifact.
* `.github/workflows/release.yml` runs on a `v*` tag: it builds the signed
  release APK and publishes it as a GitHub Release.

The release is signed with a stable upload key kept outside git, in the
`BP_KEYSTORE_BASE64` and `BP_KEYSTORE_PASSWORD` repository secrets (alias
`badproxy`). Without it the release build falls back to the debug key, which
differs per machine — those APKs cannot upgrade each other. Back the keystore
up: losing it means every installed app has to be reinstalled.

Version numbers live in `android/app/build.gradle.kts`; the tag is only used
for the release name, so bump `versionCode`/`versionName` before tagging.

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

### Emulator note

Not an app bug, but it will waste an hour: after enough VPN connect/disconnect
cycles the Android emulator leaks network state and *every* socket the app opens
outside the tunnel fails, including a plain `java.net.Socket`. `adb reboot`
clears it. If the direct half of a split mode suddenly stops working, restart
the emulator before debugging.
