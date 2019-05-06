package main

import (
	"context"
	"log"
	"time"

	"github.com/bakins/grpc-client-transcode/proxy"
	"google.golang.org/grpc"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
)

func main() {

	p, err := proxy.New("http://localhost:8080/")
	if err != nil {
		log.Fatalf("NewProxy failed: %v", err)
	}

	conn, err := p.NewConn(grpc.WithInsecure())
	if err != nil {
		log.Fatalf("NewConn failed: %v", err)
	}

	go func() {
		if err := p.Serve(context.Background()); err != nil {
			log.Fatalf("Serve failed: %v", err)
		}
	}()

	defer p.GracefulStop()

	c := pb.NewGreeterClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: "world\nhow are you?\n"})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("Greeting: %s", r.Message)
}
