module github.com/emcfarlane/larking

go 1.12

require (
	github.com/bazelbuild/buildtools v0.0.0-20220215100907-23e2a9e4721a
	github.com/emcfarlane/starlarkassert v0.0.0-20220307024619-90d731ae6256
	github.com/emcfarlane/starlarkproto v0.0.0-20210611214320-8feef53c0c82
	github.com/go-logr/logr v1.2.0
	github.com/go-logr/stdr v1.2.0
	github.com/go-openapi/spec v0.20.4
	github.com/google/go-cmp v0.5.6
	github.com/iancoleman/strcase v0.2.0
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/peterh/liner v1.2.1
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8
	github.com/soheilhy/cmux v0.1.5
	go.starlark.net v0.0.0-20220302181546-5411bad688d1
	gocloud.dev v0.24.0
	golang.org/x/mod v0.5.1 // indirect
	golang.org/x/net v0.0.0-20211123203042-d83791d6bcd9
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20220227234510-4e6760a101f9 // indirect
	golang.org/x/tools v0.1.7 // indirect
	google.golang.org/genproto v0.0.0-20211118181313-81c1377c94b1
	google.golang.org/grpc v1.42.0
	google.golang.org/protobuf v1.27.1
	modernc.org/ccgo/v3 v3.12.49 // indirect
	modernc.org/sqlite v1.13.3
	nhooyr.io/websocket v1.8.7
)

replace github.com/bazelbuild/buildtools => github.com/emcfarlane/buildtools v0.0.0-20220216022904-2d8ccb57d4be

//replace github.com/bazelbuild/buildtools => ../../bazelbuild/buildtools
