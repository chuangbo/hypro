package hypro

import (
	"log"
	"net"

	"github.com/pkg/errors"
)

type listener struct {
	reqConns <-chan net.Conn
	done     chan struct{}
}

// Listener returns net.Listener which accepts connection from hypro server
func (c *Client) Listener() (net.Listener, error) {
	if c.reqConns == nil {
		return nil, errors.New("could not create listener from non-connected client")
	}
	return &listener{
		reqConns: c.reqConns,
		done:     make(chan struct{}),
	}, nil
}

func (l *listener) Accept() (net.Conn, error) {
	log.Println("hypro.listener.Accept: waiting connection")
	select {
	case c := <-l.reqConns:
		log.Println("hypro.listener.Accept: accepted new connection")
		return c, nil
	case <-l.done:
		return nil, errors.New("hypro.listener.Accept: tunnel closed")
	}
}

// Close the listener
func (l *listener) Close() error {
	log.Println("close listener")
	close(l.done)
	return nil
}

// Addr returns net.Addr
func (l *listener) Addr() net.Addr {
	return nil
}
