package wsrpc

import (
	"log"
	"reflect"
)

type (
	// ServiceInfo lists all methods available for all services on the server.
	ServiceInfo struct {
		Methods []MethodInfo
		About   interface{} // metadata specified in ServiceDescriptor when registering service.
	}

	// ServiceDescriptor describes an RPC service specification.
	ServiceDescriptor struct {
		Name string
		// Pointer to the service interface used to check whether the user
		// provided implementation satisfies the interface requirements.
		Type    interface{}
		Methods []MethodMap
		About   interface{}
	}

	// ServiceMap matches a service implementation to its methods.
	ServiceMap struct {
		service interface{}
		methods map[string]*MethodMap
		about   interface{}
	}
)

// RegisterService registers a service and its implementation to the WebSocket
// server. It is called from generated code. All services should be registered
// before the server begins handling requests.
func (s *Server) RegisterService(sd *ServiceDescriptor, implementation interface{}) {
	requiredType := reflect.TypeOf(sd.Type).Elem()
	givenType := reflect.TypeOf(implementation)

	if !givenType.Implements(requiredType) {
		log.Fatalf("wsrpc: Server.RegisterService found the handler of type %v that does not satisfy %v", givenType, requiredType)
	}
	s.addService(sd, implementation)
}

// addService adds a service implementation to the server.
func (s *Server) addService(sd *ServiceDescriptor, implementation interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		log.Fatalf("wsrpc: Server.RegisterService after Server.Handle for %q", sd.Name)
	}
	if _, ok := s.services[sd.Name]; ok {
		log.Fatalf("wsrpc: Server.RegisterService found duplicate service registration for %q", sd.Name)
	}
	srv := &ServiceMap{
		service: implementation,
		methods: make(map[string]*MethodMap),
		about:   sd.About,
	}
	for i := range sd.Methods {
		method := &sd.Methods[i]
		srv.methods[method.Name] = method
	}
	s.services[sd.Name] = srv
}

// GetServiceInfo returns a map from service names to ServiceInfo. Service names
// include the package names, in the form of <package>.<service>.
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
