package hypro // import "github.com/chuangbo/hypro"

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	pb "github.com/chuangbo/hypro/protos"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	// maxWaitingConnections is same as Chrome's max connections per site
	maxWaitingConnections = 15
)

// Client is a reverse proxy listen on hypro grpc tunnel
type Client struct {
	Domain           string
	Server, CertFile string
	ServerPort       int
	Insecure         bool

	token string
	gc    *grpc.ClientConn
	tc    pb.TunnelClient

	reqConns chan net.Conn
}

// Dial connects hypro server at domain:port
func Dial(domain string, port int) (*Client, error) {
	c := &Client{Domain: domain, ServerPort: port}
	return c, c.Dial()
}

// Dial connects hypro server at domain:port
func (c *Client) Dial() error {
	// TODO: tls
	serverAddr := fmt.Sprintf("%s:%d", c.Server, c.ServerPort)
	// log.Println("serverAddr", serverAddr)

	var opt grpc.DialOption

	if c.Insecure {
		opt = grpc.WithInsecure()
	} else {
		creds, err := c.GetTransportCredentials()
		if err != nil {
			return errors.Wrapf(err, "could not get transport credentials")
		}

		opt = grpc.WithTransportCredentials(creds)
	}

	conn, err := grpc.Dial(serverAddr, opt)
	if err != nil {
		return errors.Wrapf(err, "failed to connect server %s", serverAddr)
	}

	c.gc = conn
	c.tc = pb.NewTunnelClient(conn)

	if err := c.Register(); err != nil {
		return err
	}

	if c.reqConns == nil {
		c.reqConns = make(chan net.Conn)
	}

	return nil
}

// Close closes the connection to the server
func (c *Client) Close() error {
	return c.gc.Close()
}

// GetTransportCredentials returns tls credentials from cert file or system root ca
func (c *Client) GetTransportCredentials() (creds credentials.TransportCredentials, err error) {
	if c.CertFile != "" {
		creds, err = credentials.NewClientTLSFromFile(c.CertFile, "")
	} else {
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		creds = credentials.NewClientTLSFromCert(rootCAs, "")
	}
	return
}

// Worker creates tunnels
func (c *Client) Worker(errCh chan<- error) {
	for {
		if err := c.CreateTunnel(); err != nil {
			errCh <- err
			return
		}
	}
}

// DialAndServe dials to the hypro server domain:port and then
// serve on handler on the hypro tunnel Listener
func DialAndServe(domain string, port int, handler http.Handler) error {
	c := &Client{Domain: domain, ServerPort: port}
	return c.DialAndServe(handler)
}

// DialAndServe connect to hypro grpc server to receive http request, and serve handler
func (c *Client) DialAndServe(handler http.Handler) error {
	if err := c.Dial(); err != nil {
		return err
	}
	defer c.Close()

	errCh := make(chan error)

	// create tunnel loop
	for i := 0; i < maxWaitingConnections; i++ {
		go c.Worker(errCh)
	}

	l, err := c.Listener()
	if err != nil {
		return errors.Wrap(err, "could not create listener")
	}
	// start the http server
	go func() {
		if err := http.Serve(l, handler); err != nil {
			errCh <- errors.Wrap(err, "could not serve reverse proxy")
		}
	}()

	log.Printf("The server is listen on: http://%s/", c.Domain)

	return <-errCh
}

// DialAndServeReverseProxy dials to the hypro server domain:port and then
// starts httputil.NewSingleHostReverseProxy on the hypro tunnel Listener
func DialAndServeReverseProxy(server string, port int, certFile string, domain, target string, insecure bool) error {
	c := &Client{
		Server:     server,
		Domain:     domain,
		ServerPort: port,
		CertFile:   certFile,
		Insecure:   insecure,
	}
	return c.DialAndServeReverseProxy(target)
}

// DialAndServeReverseProxy connect to hypro grpc server to receive http request,
// and serve as reverse proxy to the target
func (c *Client) DialAndServeReverseProxy(target string) error {
	if target == "" {
		return errors.New("target did not specific")
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		return errors.Wrapf(err, "target url invalid %s", target)
	}

	return c.DialAndServe(httputil.NewSingleHostReverseProxy(targetURL))
}

// Shutdown the server gracefully
func (c *Client) Shutdown() error {
	return errors.New("shutdown not implemented")
}

// Register the sub domain at the server.
func (c *Client) Register() error {
	log.Println("Registering", c.Domain)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r, err := c.tc.Register(ctx, &pb.RegisterRequest{Domain: c.Domain})
	if err != nil {
		return errors.Wrapf(err, "could not register %s", c.Domain)
	}
	log.Println(r)
	c.token = r.Token
	return nil
}

// CreateTunnel connects to the server and forward to reverse proxy as new net.Conn
func (c *Client) CreateTunnel() error {
	log.Println("create tunnel")
	errCh := make(chan error)

	creds := &grpcAuth{host: &c.Domain, token: &c.token, insecure: &c.Insecure}
	stream, err := c.tc.CreateTunnel(context.Background(), grpc.PerRPCCredentials(creds))

	if err != nil {
		return errors.Wrap(err, "could not create tunnel")
	}

	p1, p2 := net.Pipe()

	go func() {
		defer log.Println("tunnel closed")
		defer close(errCh)

		log.Println("start recv loop")
		defer log.Println("recv loop stopped")
		defer p1.Close()

		accepted := false

		for {
			packet, err := stream.Recv()
			// log.Println("Received", packet, err)
			if err == io.EOF {
				log.Println("recv eof")
				return
			}
			if err != nil {
				log.Println("could not recv from stream:", err)
				errCh <- err
				return
			}
			if !accepted {
				accepted = true
				c.reqConns <- p2
			}
			log.Println("Received", len(packet.Data))
			if len(packet.Data) > 0 {
				// log.Println("writing", packet.Data)
				nw, err := p1.Write(packet.Data)
				if err != nil {
					log.Println("could not write to pipe:", err)
					return
				}
				if nw != len(packet.Data) {
					// TODO: close with error
					log.Println("could not write all to pipe:", io.ErrShortWrite)
					return
				}
			}
		}
	}()

	go func() {
		log.Println("start send loop")
		defer log.Println("send loop stopped")

		buf := make([]byte, 32*1024)

		for {
			nr, err := p1.Read(buf)
			if nr > 0 {
				packet := &pb.Packet{Data: buf[0:nr]}
				err := stream.Send(packet)
				if err == io.EOF {
					return
				}
				if err != nil {
					log.Println("send error:", err)
					errCh <- err
					return
				}
			}
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Println("could not read from pipe:", err)
				return
			}
		}
	}()

	return <-errCh
}
