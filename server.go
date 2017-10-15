// Package wsrpc is a naive websocket variant of grpc.
package wsrpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"sync"
)

// Server is a websocket server for RPC requests.
type Server struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	request    chan *Request
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	services   map[string]*service // service name -> service info
	active     bool                // whether server is serving
	ctx        context.Context
	cancel     context.CancelFunc
}

type (
	// Request from browser as bytes along with WebSocket client the browser
	// communicated through.
	Request struct {
		Client  *Client
		Message []byte
	}

	// RequestHandler processes a socket request and returns a response that
	// should be sent to the client or nil if no response is expected.
	RequestHandler func(req *Request) []byte
)

const prefix = "Sec-Websocket-"

const (
	Accept   = prefix + "Accept"
	Key      = prefix + "Key"
	Protocol = prefix + "Protocol"
	Version  = prefix + "Version"
)

var (
	// ErrServerStopped indicates that the operation is now illegal because of
	// the server being stopped.
	ErrServerStopped = errors.New("wsrpc: the server has been stopped")
)

// NewServer creates a websocket server which has no service registered and has
// not started to accept requests yet.
func NewServer(c Config) *Server {
	s := &Server{
		broadcast:  make(chan []byte),
		request:    make(chan *Request),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		services:   make(map[string]*service),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())

	return s
}

// Handle incoming websocket requests. Create a client object for each
// connection with a read and write event loop.
//
// Having pumps in goroutines allows "collection of memory referenced by the
// caller" according to
//
// https://github.com/gorilla/websocket/commit/ea4d1f681babbce9545c9c5f3d5194a789c89f5b
func (s *Server) Handle(responder RequestHandler) func(w http.ResponseWriter, r *http.Request) {

	go s.listen(responder)

	// return standard HTTP handler that upgrades to socket connection
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				fmt.Fprintf(os.Stderr, "Panic: %+v\n", rvr)
				debug.PrintStack()
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()

		conn, err := upgrader.Upgrade(w, r, nil)

		if err != nil {
			log.Println(err)
			//http.Error(w, fmt.Sprintf("cannot upgrade: %v", err), http.StatusInternalServerError)
			return
		}
		client := &Client{conn: conn, server: s, Send: make(chan []byte, 256)}
		s.register <- client

		go client.writePump()
		go client.readPump()
	}
}

// remove closes the client Send channel and removes it from the server map.
func (s *Server) remove(c *Client) {
	close(c.Send)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, c)
}

// listen is an event loop that continually checks event channels.
func (s *Server) listen(responder RequestHandler) {
	for {
		select {
		case c := <-s.register:
			s.clients[c] = true

		case c := <-s.unregister:
			if _, ok := s.clients[c]; ok {
				s.remove(c)
				close(c.Send)
			}

		case req := <-s.request:
			res := responder(req)

			if res != nil {
				req.Client.Send <- res
			}

		case res := <-s.broadcast:
			for c := range s.clients {
				select {
				case c.Send <- res:
				default:
					s.remove(c)
				}
			}
		}
	}
}

// Broadcast puts a message onto the broadcast channel to be sent to all
// connected clients.
func (s *Server) Broadcast(res []byte) {
	if s.broadcast != nil && res != nil {
		s.broadcast <- res
	}
}
