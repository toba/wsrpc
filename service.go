package wsrpc

import (
	"log"
	"reflect"
)

// ServiceInfo contains unary RPC method info and metadata for a service.
type ServiceInfo struct {
	Methods []MethodInfo
	About   interface{} // metadata specified in ServiceDesc when registering service.
}

// ServiceDesc represents an RPC service's specification.
type ServiceDesc struct {
	ServiceName string
	// The pointer to the service interface used to check whether the user
	// provided implementation satisfies the interface requirements.
	HandlerType interface{}
	Methods     []MethodDesc
	About       interface{}
}

// service consists of the information of the server serving this service and
// the methods in this service.
type service struct {
	server  interface{}
	methods map[string]*MethodDesc
	about   interface{}
}

// RegisterService registers a service and its implementation to the websocket
// server. It is called from the IDL generated code. It should be called before
// invoking Handle.
func (s *Server) RegisterService(sd *ServiceDesc, ss interface{}) {
	ht := reflect.TypeOf(sd.HandlerType).Elem()
	st := reflect.TypeOf(ss)

	if !st.Implements(ht) {
		log.Fatalf("wsrpc: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
	}
	s.addService(sd, ss)
}

func (s *Server) addService(sd *ServiceDesc, ss interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		log.Fatalf("wsrpc: Server.RegisterService after Server.Serve for %q", sd.ServiceName)
	}
	if _, ok := s.services[sd.ServiceName]; ok {
		log.Fatalf("wsrpc: Server.RegisterService found duplicate service registration for %q", sd.ServiceName)
	}
	srv := &service{
		server:  ss,
		methods: make(map[string]*MethodDesc),
		about:   sd.About,
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		srv.methods[d.MethodName] = d
	}
	s.services[sd.ServiceName] = srv
}

// GetServiceInfo returns a map from service names to ServiceInfo.
// Service names include the package names, in the form of <package>.<service>.
func (s *Server) GetServiceInfo() map[string]ServiceInfo {
	ret := make(map[string]ServiceInfo)
	for n, srv := range s.services {
		methods := make([]MethodInfo, 0, len(srv.methods))
		for m := range srv.methods {
			methods = append(methods, MethodInfo{
				Name:           m,
				IsClientStream: false,
				IsServerStream: false,
			})
		}

		ret[n] = ServiceInfo{
			Methods: methods,
			About:   srv.about,
		}
	}
	return ret
}
