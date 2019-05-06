package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	gw "github.com/bakins/grpc-client-transcode/server/helloworld"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func main() {
	mux := runtime.NewServeMux()

	svr := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: mux,
	}

	client, server := net.Pipe()

	s := grpc.NewServer()
	gw.RegisterGreeterServer(s, &hello{})

	dialer := func(string, time.Duration) (net.Conn, error) {
		return client, nil
	}

	conn, err := grpc.Dial("", grpc.WithDialer(dialer), grpc.WithInsecure())

	if err != nil {
		log.Fatalf("client dial failed: %v", err)
	}

	go func() {
		if err := s.Serve(&singleListener{server}); err != nil {
			log.Fatalf("grpc server failed: %v", err)
		}
	}()

	if err := gw.RegisterGreeterHandler(context.Background(), mux, conn); err != nil {
		log.Fatalf("failed to register handle: %v", err)
	}

	if err := svr.ListenAndServe(); err != nil {
		log.Fatalf("http server failed: %v", err)
	}
}

// server is used to implement helloworld.GreeterServer.
type hello struct{}

// SayHello implements helloworld.GreeterServer
func (s *hello) SayHello(ctx context.Context, in *gw.HelloRequest) (*gw.HelloReply, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		for k, v := range md {
			for _, val := range v {
				fmt.Println(k, val)
			}
		}
	}

	log.Printf("Received: %v", in.Name)
	return &gw.HelloReply{Message: "Hello " + in.Name}, nil
}

// https://github.com/glerchundi/grpc-boomerang
type singleListener struct {
	conn net.Conn
}

func (s *singleListener) Accept() (net.Conn, error) {
	fmt.Println("I am here")
	if c := s.conn; c != nil {
		fmt.Println("returning conn")
		s.conn = nil
		return c, nil
	}
	// another accept should block indefinitley. TODO: use a context
	select {}
	return nil, io.EOF
}

func (s *singleListener) Close() error {
	return nil
}

func (s *singleListener) Addr() net.Addr {
	return s.conn.LocalAddr()
}
