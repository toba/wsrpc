// https://www.jonathan-petitcolas.com/2015/01/27/playing-with-websockets-in-go.html
// https://github.com/gorilla/websocket/tree/master/examples
package wsrpc

import (
	"log"
	"net/http"
	"time"

	"strings"

	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

// Client represents a connected browser.
type Client struct {
	conn *websocket.Conn
	// Buffered channel of outbound messages to be picked up by the writePump.
	Send   chan []byte
	server *Server
	Token  *oauth2.Token
}

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CheckOrigin ensures client is allowed to connect.
	// http://www.gorillatoolkit.org/pkg/websocket
	CheckOrigin: func(r *http.Request) bool {
		return strings.HasPrefix(r.RemoteAddr, "127.0.0.1") || r.Header["Origin"][0] == r.Host
	},
}

// readPump processes messages from the client connection.
//
// It executes in one goroutine per client ensuring there is only one reader
// per connection.
func (c *Client) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}
		c.server.request <- &Request{
			Client:     c,
			RawMessage: message,
			WireLength: len(message),
		}
	}
}

// writePump sends messages to connected clients.
//
// It executes in one goroutine per client ensuring there is only one writer
// per connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case res, ok := <-c.Send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel has been closed.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(res)

			// Add queued messages to the current websocket message.
			// n := len(c.Send)
			// for i := 0; i < n; i++ {
			// 	w.Write(newline)
			// 	w.Write(<-c.Send)
			// }

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}
