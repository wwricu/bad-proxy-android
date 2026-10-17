module github.com/wwricu/bad-proxy-android

go 1.26.3

require (
	github.com/wwricu/bad-proxy-core v0.0.0-00010101000000-000000000000
	github.com/xjasonlyu/tun2socks/v2 v2.7.0
	golang.org/x/sys v0.49.0
	gvisor.dev/gvisor v0.0.0-20260701204157-69c2d17aea96
)

require (
	github.com/google/btree v1.1.3 // indirect
	golang.org/x/exp v0.0.0-20260611194520-c48552f49976 // indirect
	golang.org/x/mobile v0.0.0-20261011034528-197770444867 // indirect
	golang.org/x/time v0.15.0 // indirect
)

replace github.com/wwricu/bad-proxy-core => ../bad-proxy-core
