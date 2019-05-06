package proxy

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Proxy struct {
	client   net.Conn
	server   net.Conn
	grpc     *grpc.Server
	endpoint *url.URL
	http     *http.Client
}

func New(endpoint string) (*Proxy, error) {
	client, server := net.Pipe()

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse endpoint")
	}

	p := &Proxy{
		client:   client,
		server:   server,
		endpoint: u,
		http:     &http.Client{},
	}

	p.grpc = grpc.NewServer(
		grpc.CustomCodec(Codec()),
		grpc.UnknownServiceHandler(p.streamHandler),
	)

	return p, nil
}

// Serve starts the internal proxy. Should be called in a goroutine
func (p *Proxy) Serve(ctx context.Context) error {
	return p.grpc.Serve(&singleListener{conn: p.server, ctx: ctx})
}

func (p *Proxy) GracefulStop() {
	p.grpc.GracefulStop()
}

func (p *Proxy) Stop() {
	p.grpc.Stop()
}

func (p *Proxy) NewConn(options ...grpc.DialOption) (*grpc.ClientConn, error) {
	dialer := func(context.Context, string) (net.Conn, error) {
		return p.client, nil
	}

	defaultOptions := []grpc.DialOption{
		grpc.WithContextDialer(dialer),
		grpc.WithDefaultCallOptions(
			grpc.ForceCodec(&jsonpbCodec{}),
		),
	}

	options = append(options, defaultOptions...)

	conn, err := grpc.Dial("", options...)
	if err != nil {
		return nil, errors.Wrap(err, "dial failed")
	}
	return conn, nil
}

func (p *Proxy) streamHandler(srv interface{}, stream grpc.ServerStream) error {
	lowLevelServerStream := grpc.ServerTransportStreamFromContext(stream.Context())
	if lowLevelServerStream == nil {
		return status.Errorf(codes.Internal, "lowLevelServerStream does not exist in context")
	}

	fullMethodName := lowLevelServerStream.Method()

	clientCtx, clientCancel := context.WithCancel(stream.Context())
	defer clientCancel()

	var f frame
	if err := stream.RecvMsg(&f); err != nil {
		return status.Errorf(codes.Internal, "RecvMsg failed: %s", err)
	}

	// copy the url
	u := *p.endpoint
	u.Path = fullMethodName

	req := &http.Request{
		Method:        http.MethodPost,
		URL:           &u,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		ContentLength: int64(len(f.payload)),
		Body:          ioutil.NopCloser(bytes.NewBuffer(f.payload)),
		Host:          u.Host,
	}

	md, ok := metadata.FromIncomingContext(clientCtx)
	if ok {
		for k, v := range md {
			if shouldSkipHeader(k) {
				continue
			}
			k = strings.TrimPrefix(k, ":")
			for _, val := range v {
				req.Header.Add(runtime.MetadataHeaderPrefix+k, val)
			}
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	req = req.WithContext(clientCtx)

	resp, err := p.http.Do(req)

	if err != nil {
		return status.Errorf(codes.Internal, "http request failed: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return status.Errorf(codes.Internal, "unexpected HTTP status: %d", resp.StatusCode)
	}

	responseMetadata := metadata.MD{}

	for k, v := range resp.Header {
		// this probably need to be munged?
		responseMetadata[strings.ToLower(k)] = v
	}

	if err := stream.SendHeader(responseMetadata); err != nil {
		return status.Errorf(codes.Internal, "failed to send headers: %v", err)
	}
	reader := bufio.NewReader(resp.Body)

	for {
		// gateway returns streams as json with newlines
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to read response body: %v", err)
		}

		f := frame{
			payload: line,
		}
		if err := stream.SendMsg(&f); err != nil {
			return status.Errorf(codes.Internal, "failed to send message: %v", err)
		}
	}

	return nil

}

// https://github.com/glerchundi/grpc-boomerang
type singleListener struct {
	conn net.Conn
	ctx  context.Context
}

func (s *singleListener) Accept() (net.Conn, error) {
	if c := s.conn; c != nil {
		s.conn = nil
		return c, nil
	}
	<-s.ctx.Done()
	return nil, io.EOF
}

func (s *singleListener) Close() error {
	return nil
}

func (s *singleListener) Addr() net.Addr {
	return s.conn.LocalAddr()
}

var skipHeaders = map[string]bool{
	"content-type": true,
	":authority":   true,
}

func shouldSkipHeader(k string) bool {
	return skipHeaders[k]
}
