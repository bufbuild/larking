# [larking.io](https://larking.io)

[![Go Reference](https://pkg.go.dev/badge/github.com/emcfarlane/larking.svg)](https://pkg.go.dev/github.com/emcfarlane/larking)

Reflective gRPC transcoding handler. Get started: [larking.io/docs](https://larking.io/docs)

- [Transcoding protobuf descriptors REST/HTTP to gRPC](https://cloud.google.com/endpoints/docs/grpc/transcoding)
- [Follows Google API Design principles](https://cloud.google.com/apis/design)
- [Dynamically load descriptors via gRPC server reflection](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md)

<div align="center">
<img src="docs/larking.svg" />
</div>


## Install

```
go get github.com/emcfarlane/larking@latest
```

### Install the REPL

```
go install github.com/emcfarlane/larking/cmd/lark@latest
```

### Install the worker

```
go install github.com/emcfarlane/larking/cmd/larking@latest
```

## Developing

### Debugging

Checkout [protobuf](https://github.com/golang/protobuf) at the latest v2 relase.
Go install each protoc generation bin.

Regenerate protoc buffers:

```
protoc --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. testpb/test.proto
```

### Protoc

Must have googleapis protos avaliable.
Just link API to `/usr/local/include/google` so protoc can find it.
```
ln -s ~/src/github.com/googleapis/googleapis/google/api/ /usr/local/include/google/
```
