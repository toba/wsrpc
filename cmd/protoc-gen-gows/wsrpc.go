package main

import (
	"fmt"
	"path"
	"strconv"

	pb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
)

// Paths for packages used by code generated in this file relative to the
// import_prefix of the generator.Generator.
const (
	contextPkgPath = "context"
	wsRpcPkgPath   = "github.com/toba/wsrpc"
	gRpcPkgPath    = "google.golang.org/grpc"
)

var (
	contextPkg string
	wsRpcPkg   string
	gRpcPkg    string
)

func init() {
	generator.RegisterPlugin(new(wsRPC))
}

// wsRPC is implemented as a Go protocol buffer plugin to generate bindings
// for WebSocket RPC.
type wsRPC struct {
	gen *generator.Generator
}

// Name of this plugin.
func (ws *wsRPC) Name() string {
	return "wsrpc"
}

// Init initializes the protoc plugin.
func (ws *wsRPC) Init(gen *generator.Generator) {
	ws.gen = gen
	contextPkg = generator.RegisterUniquePackageName("context", nil)
	wsRpcPkg = generator.RegisterUniquePackageName("wsrpc", nil)
	gRpcPkg = generator.RegisterUniquePackageName("grpc", nil)
}

// Given a type name defined in a .proto, return its object.
// Also record that we're using it, to guarantee the associated import.
func (ws *wsRPC) objectNamed(name string) generator.Object {
	ws.gen.RecordTypeUse(name)
	return ws.gen.ObjectNamed(name)
}

// Given a type name defined in a .proto, return its name as we will print it.
func (ws *wsRPC) typeName(str string) string {
	return ws.gen.TypeName(ws.objectNamed(str))
}

// P forwards to g.gen.P.
func (ws *wsRPC) P(args ...interface{}) { ws.gen.P(args...) }

// Generate WebSocket code for the services in the given file and for the
// methods defined in each service.
func (ws *wsRPC) Generate(file *generator.FileDescriptor) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}

	ws.P("// Reference imports to suppress errors if not otherwise used.")
	ws.P("var (")
	ws.P("_ ", contextPkg, ".Context")
	ws.P("_ ", wsRpcPkg, ".Client")
	ws.P("_ ", gRpcPkg, ".ClientConn")
	ws.P(")")
	ws.P()

	for i, service := range file.FileDescriptorProto.Service {
		ws.generateService(file, service, i)
	}
}

// GenerateImports generates the import declaration for this file.
func (ws *wsRPC) GenerateImports(file *generator.FileDescriptor) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}
	ws.P("import (")
	ws.P(contextPkg, " ", strconv.Quote(path.Join(ws.gen.ImportPrefix, contextPkgPath)))
	ws.P(wsRpcPkg, " ", strconv.Quote(path.Join(ws.gen.ImportPrefix, wsRpcPkgPath)))
	ws.P(gRpcPkg, " ", strconv.Quote(path.Join(ws.gen.ImportPrefix, gRpcPkgPath)))
	ws.P(")")
	ws.P()
}

// generateService generates WebSocket handles for the named service.
func (ws *wsRPC) generateService(file *generator.FileDescriptor, service *pb.ServiceDescriptorProto, index int) {
	path := fmt.Sprintf("6,%d", index) // 6 means service.

	originalName := service.GetName()
	fullName := originalName
	if pkg := file.GetPackage(); pkg != "" {
		fullName = pkg + "." + fullName
	}
	name := generator.CamelCase(originalName)
	descriptor := name + "ServiceDescriptor"
	serviceType := name + "Service"

	ws.P("// WebSocket Server API for ", name, " service")
	ws.P()
	ws.P("type ", serviceType, " interface {")

	for i, method := range service.Method {
		ws.gen.PrintComments(fmt.Sprintf("%s,2,%d", path, i)) // 2 means method in a service
		ws.P(ws.generateServerSignature(name, method))
	}
	ws.P("}")
	ws.P()

	// Service registration.
	ws.P("func Register", name, "Service(s *", wsRpcPkg, ".Server, srv ", serviceType, ") {")
	ws.P("s.RegisterService(&", descriptor, `, srv)`)
	ws.P("}")
	ws.P()

	// Service endpoints.
	var handlerNames []string
	for _, method := range service.Method {
		hname := ws.generateServiceMethod(name, fullName, method)
		handlerNames = append(handlerNames, hname)
	}

	// Service descriptor.
	ws.P("var ", descriptor, " = ", wsRpcPkg, ".ServiceDescriptor {")
	ws.P("Name: ", strconv.Quote(fullName), ",")
	ws.P("HandlerType: (*", serviceType, ")(nil),")
	ws.P("Methods: []", wsRpcPkg, ".MethodMap{")
	for i, method := range service.Method {
		if method.GetServerStreaming() || method.GetClientStreaming() {
			continue
		}
		ws.P("{")
		ws.P("Name: ", strconv.Quote(method.GetName()), ",")
		ws.P("Handler: ", handlerNames[i], ",")
		ws.P("},")
	}
	ws.P("},")
	ws.P("About: \"", file.GetName(), "\",")
	ws.P("}")
	ws.P()
}

// generateServerSignature returns the server-side signature for a method.
func (ws *wsRPC) generateServerSignature(servName string, method *pb.MethodDescriptorProto) string {
	return generator.CamelCase(method.GetName()) +
		"(" + contextPkg + ".Context" + ") " +
		"(*" + ws.typeName(method.GetOutputType()) + ", error)"
}

// generateServiceMethod creates Go code
func (ws *wsRPC) generateServiceMethod(serviceName, fullServName string, method *pb.MethodDescriptorProto) string {
	methodName := generator.CamelCase(method.GetName())
	fullName := fmt.Sprintf("%s%sHandler", serviceName, methodName)
	inType := ws.typeName(method.GetInputType())

	ws.P("func ", fullName, "(srv interface{}, ctx ", contextPkg, ".Context, decode func(interface{}) error) (interface{}, error) {")
	ws.P("in := &", inType, "{}")
	ws.P("if err := decode(in); err != nil { return nil, err }")
	ws.P("return srv.(", serviceName, "Service).", methodName, "(ctx, in)")
	ws.P("}")
	ws.P()

	return fullName
}
