package wsrpc

import "context"

// MethodInfo describes an RPC endpoint.
type MethodInfo struct {
	Name           string
	IsClientStream bool
	IsServerStream bool
}

type methodHandler func(
	service interface{},
	ctx context.Context,
	decoder func(interface{}) error) (interface{}, error)

// MethodMap maps a fully qualified method name to its handler.
type MethodMap struct {
	Name    string
	Handler methodHandler
}
