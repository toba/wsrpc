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
	dec func(interface{}) error) (interface{}, error)

// MethodDesc represents an RPC service's method specification.
type MethodDesc struct {
	MethodName string
	Handler    methodHandler
}
