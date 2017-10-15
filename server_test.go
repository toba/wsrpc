package wsrpc_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/toba/wsrpc"
)

var (
	c     = wsrpc.Config{}
	hello = []byte("hello")
	world = []byte("world")
)

// https://play.golang.org/p/X8GLU-Gcox
func connect(t *testing.T, h wsrpc.RequestHandler) *websocket.Conn {
	rpc := wsrpc.NewServer(c)
	handler := rpc.Handle(h)
	srv := httptest.NewServer(http.HandlerFunc(handler))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	conn, res, err := websocket.DefaultDialer.Dial(u.String(), nil)

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, conn)

	assert.Equal(t, http.StatusSwitchingProtocols, res.StatusCode)
	assert.Contains(t, res.Header, wsrpc.Accept)

	return conn
}

func mockHandler(t *testing.T) wsrpc.RequestHandler {
	return func(req *wsrpc.Request) []byte {
		assert.NotNil(t, req)
		assert.Equal(t, hello, req.Message)
		return world
	}
}

func TestServiceMessage(t *testing.T) {
	conn := connect(t, mockHandler(t))

	defer conn.Close()

	err := conn.WriteMessage(websocket.BinaryMessage, hello)
	assert.NoError(t, err)

	messageType, res, err := conn.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.Equal(t, world, res)
}
