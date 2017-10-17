// Package wsrpc is a naive WebSocket variant of gRPC.
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
	"time"

	"google.golang.org/grpc"
)

// Server is a WebSocket server for RPC requests. The gRPC server assigns the
// codec per client connection but this implementation always uses protobuf so
// it can be defined at the server level.
type Server struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	request    chan *Request
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	services   map[string]*ServiceMap // service name -> service info
	active     bool                   // whether server is processing requests
	codec      grpc.Codec
	ctx        context.Context
	cancel     context.CancelFunc
}

type (
	// Request from browser as bytes along with WebSocket client the browser
	// communicated through.
	Request struct {
		Client     *Client
		ReceivedAt time.Time
		RawMessage []byte
		WireLength int
		Message    interface{} // decoded proto message
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

// NewServer creates a websocket server which has no service registered and is
// not yet accepting requests.
func NewServer(c Config) *Server {
	s := &Server{
		broadcast:  make(chan []byte),
		request:    make(chan *Request),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		services:   make(map[string]*ServiceMap),
		codec:      protoCodec{},
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())

	return s
}

// Handle incoming WebSocket requests. Create a client object for each
// connection with a read and write event loop.
func (s *Server) Handle() func(w http.ResponseWriter, r *http.Request) {

	go s.listen()

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
//
// The request case is equivalent to the gRPC clientStream.RecvMsg() or
// Server.processUnaryRPC() which decode a binary message.
//
// In gRPC, the service and method name are sent in the request header which
// are used to lookup the handler implementation.
func (s *Server) listen() {
	for {
		select {
		case c := <-s.register:
			s.clients[c] = true

		case c := <-s.unregister:
			if _, ok := s.clients[c]; ok {
				s.remove(c)
			}

		case req := <-s.request:
			var msg interface{}
			// TODO: source these objects since we can't get them from a header
			md := MethodMap{}
			srv := ServiceMap{}

			df := func(v interface{}) error {
				return nil
			}

			if err := s.codec.Unmarshal(req.RawMessage, msg); err != nil {
				log.Fatal("oops")
			}

			req.Message = msg
			req.ReceivedAt = time.Now()

			reply, err := md.Handler(srv.service, s.ctx, df)

			if err != nil {
				log.Fatal("oops")
			}

			res, err := s.codec.Marshal(reply)

			if err != nil {
				log.Fatal("oops")
			}

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
