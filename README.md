# grpc client HTTP/2 to HTTP/1.1 proxy

Go grpc "proxy" that transcodes grpc to JSON+HTTP/1.1

## Why?

This probably doesn't have any practical use, but it was a fun thing to write. 

I've been frustrated with all the steps needed to get a grpc service hosted in [GKE](https://cloud.google.com/kubernetes-engine/) and exposed to the public internet. It's doable, but one day while reading docs, my mind wandered and I thought "hmm, could I do this over HTTP/1.1" and wound up hacking this together.  It's not exactly grpc over HTTP/1.1.

I wanted to test this with unmodified Go grpc clients.

TL;DR - I like doing silly things with gRPC, so why not.

## What is it?

This provides a "proxy" that transcodes grpc to JSON+HTTP/1.1.  The proxy runs in process.  

The proxy was written to interact with grpc services fronted by the [grpc gateway](https://github.com/grpc-ecosystem/grpc-gateway).  [Google Cloud Endpoints](https://cloud.google.com/endpoints/docs/grpc/transcoding) also supports similar server side transcoding.

Why transcode grpc to JSON+HTTP/1.1 just to transcode it back?  As stated above, this probably has no practical usage, but if one needed to contact a grpc service but was unable to do end-to-end HTTP/2, this this may inspire some workarounds.

### General Flow

*A new proxy is created and started.
* A [client connection](https://godoc.org/google.golang.org/grpc#ClientConn) is created by the proxy.  The proxy sets [ForceCodec](https://godoc.org/google.golang.org/grpc#ForceCodec) on the connection to [JSONPb](https://godoc.org/github.com/grpc-ecosystem/grpc-gateway/runtime#JSONPb) which is the same codec used by [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) to transcode JSON<->protobuf.
* A grpc client is created using the client connection.  This example uses the [hello world client](https://github.com/grpc/grpc-go/tree/master/examples/helloworld/greeter_client).
* The client connection uses a [net.Pipe](https://golang.org/pkg/net/#Pipe) to copy requests to the grpc server in the proxy.
* The proxy grpc server uses a [CutomCodec](https://godoc.org/google.golang.org/grpc#CustomCodec) that passes that passes the data from the client through.  See [grpc proxy](https://github.com/mwitkow/grpc-proxy/blob/0f1106ef9c766333b9acb4b81e705da4bade7215/proxy/codec.go#L17) for more information on how this works.
* The proxy grpc server uses an [UnknownServiceHandler](https://github.com/mwitkow/grpc-proxy/blob/0f1106ef9c766333b9acb4b81e705da4bade7215/proxy/codec.go#L17) that uses HTTP/1.1 POST requests. It receieves the responses and passes them pack to the client.
* The client, uses the [JSONPb](https://godoc.org/github.com/grpc-ecosystem/grpc-gateway/runtime#JSONPb) to Unmarshal the response to the Go struct.

This should work with streaming responses.  It expects newline separated JSON lines like grpc gateway produces. 

## What's included?

The [proxy package](./proxy) does the heavy lifting.  It creates an in-process grpc sever that handles the transcoding and can create connections to this server that can be used by normal Go grpc clients.

The top level directory includes a sample client that uses the proxy in [main.go](./main.go).

A sample server is included in [./server](server.go)


## Inspired by

* https://github.com/glerchundi/grpc-boomerang - grpc over HTTP websockets
* https://github.com/mwitkow/grpc-proxy - grpc passthrough proxy
* https://github.com/grpc-ecosystem/grpc-gateway


## LICENSE

See [LICENSE](./LICENSE)