package hypro // import "github.com/chuangbo/hypro"

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	pb "github.com/chuangbo/hypro/protos"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

const recycleClientDelay = time.Second

var (
	errNoIdleConn = errors.New("no idle conn available")
)

// Server is
type Server struct {
	// the server needs the port to generate full address for the registered domain
	GRPCAddr, HTTPAddr, HTTPPort string

	CertFile, KeyFile string

	mu    sync.RWMutex // protects users
	users map[string]*user

	recycles chan *user

	done chan struct{}
}

type user struct {
	host, token string

	server *Server

	mu        sync.RWMutex // protects idle conns
	idleConns []net.Conn

	createdAt, lastConnAt time.Time
}

// ListenAndServe listens on grpcAddr and http on httpAddr serving hypro tunnel
// services and http forwarding to the client
func ListenAndServe(grpcAddr, httpAddr, certFile, keyFile string) error {
	server := &Server{
		GRPCAddr: grpcAddr,
		HTTPAddr: httpAddr,
		CertFile: certFile,
		KeyFile:  keyFile,
	}
	err := server.ListenAndServe()
	if err != nil {
		return errors.Wrap(err, "could not listen and serve")
	}
	return nil
}

// ListenAndServe listen on grpc addr and tcp on http addr to provide
// hypro http proxy through grpc connection with the users
func (s *Server) ListenAndServe() error {
	// grpc server
	log.Printf("Starting grpc server at %v\n", s.GRPCAddr)
	lis, err := net.Listen("tcp", s.GRPCAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to listen grpc on %s", s.GRPCAddr)
	}
	if s.HTTPAddr == "" {
		return errors.New("http addr could not be empty")
	}
	if s.users == nil {
		s.users = map[string]*user{}
	}
	if s.recycles == nil {
		s.recycles = make(chan *user)
	}
	if s.done == nil {
		s.done = make(chan struct{})
	}
	if s.HTTPPort == "" {
		_, httpPort, err := net.SplitHostPort(s.HTTPAddr)
		if err != nil {
			return errors.Wrapf(err, "could not find the http port %s", s.HTTPAddr)
		}
		s.HTTPPort = httpPort
	}

	var grpcServer *grpc.Server
	var opt grpc.ServerOption

	if s.CertFile != "" && s.KeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(s.CertFile, s.KeyFile)
		if err != nil {
			return errors.Wrap(err, "certificates invalid")
		}
		opt = grpc.Creds(creds)
	}

	if opt != nil {
		grpcServer = grpc.NewServer(opt)
	} else {
		grpcServer = grpc.NewServer()
	}

	pb.RegisterTunnelServer(grpcServer, s)
	go grpcServer.Serve(lis)
	// recycle no connection users
	go s.recycleUsers()

	// http reverse proxy
	reverseProxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = "http"
			r.URL.Host = r.Host
		},
		// replace http.DefaultTransport DialContext func to dial to virtual conn
		Transport: &http.Transport{
			DialContext:           s.DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	log.Printf("Starting http server at %v\n", s.HTTPAddr)
	if err := http.ListenAndServe(s.HTTPAddr, reverseProxy); err != nil {
		return errors.Wrapf(err, "failed to listen http on %s", s.HTTPAddr)
	}
	return nil
}

// Shutdown close the server
func (s *Server) Shutdown() error {
	close(s.done)
	return nil
}

// DialContext return a pre-connected proxy connection which actually r/w from grpc
func (s *Server) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get host from %s", addr)
	}
	log.Println(network, addr, host)
	// TODO: wait until client connected
	c, err := s.getIdleConn(host)
	if err != nil {
		return nil, errors.Wrapf(err, "tunnel not found %s", host)
	}
	log.Printf("dial new virtual connection: %p\n", c)
	return c, nil
}

// Register the client
func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	log.Println("Registering domain:", req.Domain)
	fullDomain := req.Domain

	if s.TunnelExists(req.Domain) {
		log.Println("Register: domain unavailable:", req.Domain)
		return nil, grpc.Errorf(codes.AlreadyExists, "domain %s unavailable", req.Domain)
	}

	if s.HTTPPort != "80" {
		fullDomain = fmt.Sprintf("%s:%s", req.Domain, s.HTTPPort)
	}

	token, err := generateRandomString(32)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "could not create token")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	c := &user{
		server:    s,
		token:     token,
		idleConns: []net.Conn{},
		createdAt: time.Now(),
	}
	s.users[req.Domain] = c
	log.Println("number of users", len(s.users))

	// remove token if no connection after register
	time.AfterFunc(recycleClientDelay, func() {
		s.recycles <- c
	})

	return &pb.RegisterResponse{
		FullDomain: fullDomain,
		Token:      token,
	}, nil
}

// CreateTunnel accept and keep connection between client and server
// TODO: use metadata or custom auth to bind
func (s *Server) CreateTunnel(stream pb.Tunnel_CreateTunnelServer) error {
	md, ok := metadata.FromIncomingContext(stream.Context())
	host := md["host"][0]

	if !ok || !s.Authenticated(host, md["token"][0]) {
		return grpc.Errorf(codes.Unauthenticated, "valid token required")
	}

	s.mu.RLock()
	c := s.users[host]
	s.mu.RUnlock()

	c.mu.RLock()
	if len(c.idleConns) >= maxWaitingConnections {
		c.mu.RUnlock()
		return grpc.Errorf(codes.ResourceExhausted, "reached max waiting connections %d", maxWaitingConnections)
	}
	c.mu.RUnlock()

	p1, p2 := net.Pipe()

	c.putIdleConn(p2)
	defer c.removeIdleConn(p2)

	go func() {
		defer log.Println("tunnel closed")

		log.Println("start recv loop")
		defer log.Println("recv loop stopped")
		defer p1.Close()

		for {
			packet, err := stream.Recv()
			// log.Println("Received", packet, err)
			if err == io.EOF {
				log.Println("recv eof")
				return
			}
			if err != nil {
				log.Println("could not recv from stream:", err)
				return
			}
			log.Println("Received", len(packet.Data))
			if len(packet.Data) == 0 {
				continue
			}
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
	}()

	log.Println("start send loop")
	defer log.Println("send loop stopped")

	buf := make([]byte, 32*1024)

	for {
		nr, err := p1.Read(buf)
		if nr > 0 {
			packet := &pb.Packet{Data: buf[0:nr]}
			err := stream.Send(packet)
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return errors.Wrap(err, "could not send to stream")
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			log.Println("could not read from pipe:", err)
			return nil
		}
	}
}

// TunnelExists checks if the tunnel registerred and connected
func (s *Server) TunnelExists(host string) bool {
	s.mu.RLock()
	c, ok := s.users[host]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.idleConns) > 0
}

// Authenticated checks valid token from grpc metadata
func (s *Server) Authenticated(host, token string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return host != "" && token != "" && s.users[host].token == token
}

func (s *Server) getIdleConn(host string) (net.Conn, error) {
	s.mu.RLock()
	c, ok := s.users[host]
	s.mu.RUnlock()

	if ok {
		return c.getIdleConn()
	}
	return nil, errNoIdleConn
}

func (c *user) getIdleConn() (net.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.idleConns) > 0 {
		conn := c.idleConns[0]
		c.idleConns = c.idleConns[1:]
		log.Println("number of idle conns:", len(c.idleConns))
		return conn, nil
	}
	return nil, errNoIdleConn
}

func (c *user) putIdleConn(conn net.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastConnAt = time.Now()
	c.idleConns = append(c.idleConns, conn)
	log.Println("number of idle conns:", len(c.idleConns), c.host)
}

// removeIdleConn removes the closed conn from the pool
func (c *user) removeIdleConn(conn net.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	conns := c.idleConns
	for i, v := range conns {
		if v == conn {
			conns = append(conns[:i], conns[i+1:]...)
			c.idleConns = conns
			break
		}
	}
	if len(conns) == 0 {
		time.AfterFunc(recycleClientDelay, func() {
			c.server.recycles <- c
		})
	}
	log.Println("number of idle conns:", len(conns), c.host)
}

func (s *Server) recycleUsers() {
	for {
		select {
		case c := <-s.recycles:
			c.mu.RLock()
			// the token should be recycle if the client has no idle connections,
			// and the last connection was created before 1 second ago
			if len(c.idleConns) == 0 &&
				c.lastConnAt.Before(time.Now().Add(-recycleClientDelay)) {
				s.mu.Lock()
				delete(s.users, c.host)
				s.mu.Unlock()
			}
			c.mu.RUnlock()
		case <-s.done:
			return
		}
	}
}
