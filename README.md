# grpcmix: serving gRPC, gRPC-Web, and HTTP on a single port

grpcmix is a convenient Golang library that enables you to run gRPC, gRPC-Web, and HTTP services on a single port easily.

## Motivation

In many scenarios, you may want to run both gRPC and HTTP on the same service. For example, you may want to provide gRPC services while exposing Prometheus metrics through HTTP endpoints. In other instances, you may need to convert a gRPC service into gRPC-Web to allow it to be accessed from web browsers, which typically requires additional deployment and is not straightforward.

With grpcmix, you can run all of these protocols on the same port, eliminating the need for multiple ports or extra components to be set up and deployed.

## Features

- Integrated gRPC-Web support (using [improbable-eng/grpc-web](https://github.com/improbable-eng/grpc-web))
- Single port serving for gRPC, gRPC-Web, and HTTP
- gRPC-Web with Brotli & gzip compression (using [andybalholm/brotli](https://github.com/andybalholm/brotli))
- Graceful shutdown with configurable delay

## Usage

Install grpcmix:
```shell
go get github.com/lqs/grpcmix
```

Add the following lines to your code:
```go
// create a grpcmix server
mixServer := grpcmix.NewServer(grpcmix.Config{
    Port:          8080,
    ShutdownDelay: 2 * time.Second,
    httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("add your http handler here, like prometheus metrics, health check, ..."))
    }),
})

// register your grpc services
yourservicepb.RegisterYourServiceServer(mixServer, YourServiceImpl{})

// start the server and let it gracefully shutdown on SIGTERM
ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM)
if err := mixServer.StartAndWait(ctx); err != nil {
    // server start failed
    panic(err)
}
```

With these few lines of code, you can start a production-ready server that serves gRPC (over HTTP/2), gRPC-Web (over HTTP/1.x & HTTP/2, with brotli compression), and HTTP (over HTTP/1.x & HTTP/2) on the same port.
