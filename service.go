package wsrpc

import (
	"log"
	"reflect"
)

// ServiceInfo contains method info and metadata for a service.
type ServiceInfo struct {
	Methods []MethodInfo
	About   interface{} // metadata specified in ServiceDesc when registering service.
}

// ServiceDesc represents an RPC service's specification.
type ServiceDescriptor struct {
	Name string
	// The pointer to the service interface used to check whether the user
	// provided implementation satisfies the interface requirements.
	HandlerType interface{}
	Methods     []MethodMap
	About       interface{}
}

// service consists of the information of the server serving this service and
// the methods in this service.
type service struct {
	server  interface{}
	methods map[string]*MethodMap
	about   interface{}
}

// RegisterService registers a service and its implementation to the websocket
// server. It is called from the IDL generated code. It should be called before
// invoking Handle.
func (s *Server) RegisterService(sd *ServiceDescriptor, ss interface{}) {
	ht := reflect.TypeOf(sd.HandlerType).Elem()
	st := reflect.TypeOf(ss)

	if !st.Implements(ht) {
		log.Fatalf("wsrpc: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
	}
	s.addService(sd, ss)
}

func (s *Server) addService(sd *ServiceDescriptor, ss interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		log.Fatalf("wsrpc: Server.RegisterService after Server.Serve for %q", sd.Name)
	}
	if _, ok := s.services[sd.Name]; ok {
		log.Fatalf("wsrpc: Server.RegisterService found duplicate service registration for %q", sd.Name)
	}
	srv := &service{
		server:  ss,
		methods: make(map[string]*MethodMap),
		about:   sd.About,
	}
	for i := range sd.Methods {
		method := &sd.Methods[i]
		srv.methods[method.Name] = method
	}
	s.services[sd.Name] = srv
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
