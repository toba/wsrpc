package wsrpc

import "context"

// MethodInfo describes an RPC endpoint.
type MethodInfo struct {
	Name           string // Name without service or package name.
	IsClientStream bool
	IsServerStream bool
}

type methodHandler func(
	srv interface{},
	ctx context.Context,
	decode func(interface{}) error) (interface{}, error)

// MethodMap maps a method name to its handler.
type MethodMap struct {
	Name    string
	Handler methodHandler
}
