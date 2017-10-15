package plugin

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	pb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
)

// Paths for packages used by code generated in this file relative to the
// import_prefix of the generator.Generator.
const (
	contextPkgPath = "context"
	wsRpcPkgPath   = "github.com/toba/wsrpc"
)

var (
	contextPkg string
	wsRpcPkg   string
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

// Init initializes the plugin.
func (ws *wsRPC) Init(gen *generator.Generator) {
	ws.gen = gen
	contextPkg = generator.RegisterUniquePackageName("context", nil)
	wsRpcPkg = generator.RegisterUniquePackageName("wsrpc", nil)
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

// Generate WebSocket code for the services in the given file.
func (ws *wsRPC) Generate(file *generator.FileDescriptor) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}

	ws.P("// Reference imports to suppress errors if they are not otherwise used.")
	ws.P("var _ ", contextPkg, ".Context")
	ws.P("var _ ", wsRpcPkg, ".Client")
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
	ws.P(")")
	ws.P()
}

func unexport(s string) string { return strings.ToLower(s[:1]) + s[1:] }

// generateService generates WebSocket handles for the named service.
func (ws *wsRPC) generateService(file *generator.FileDescriptor, service *pb.ServiceDescriptorProto, index int) {
	path := fmt.Sprintf("6,%d", index) // 6 means service.

	originalName := service.GetName()
	fullName := originalName
	if pkg := file.GetPackage(); pkg != "" {
		fullName = pkg + "." + fullName
	}
	name := generator.CamelCase(originalName)
	descriptor := "_" + name + "_serviceDesc"
	serverType := name + "Server"

	ws.P("// Server API for ", name, " service")
	ws.P()
	ws.P("type ", serverType, " interface {")

	for i, method := range service.Method {
		ws.gen.PrintComments(fmt.Sprintf("%s,2,%d", path, i)) // 2 means method in a service
		ws.P(ws.generateServerSignature(name, method))
	}
	ws.P("}")
	ws.P()

	// Server registration.
	ws.P("func Register", name, "Server(s *", wsRpcPkg, ".Server, srv ", serverType, ") {")
	ws.P("s.RegisterService(&", descriptor, `, srv)`)
	ws.P("}")
	ws.P()

	// Server handler implementations.
	var handlerNames []string
	for _, method := range service.Method {
		hname := ws.generateServerMethod(name, fullName, method)
		handlerNames = append(handlerNames, hname)
	}

	// Service descriptor.
	ws.P("var ", descriptor, " = ", wsRpcPkg, ".ServiceDesc {")
	ws.P("ServiceName: ", strconv.Quote(fullName), ",")
	ws.P("HandlerType: (*", serverType, ")(nil),")
	ws.P("Methods: []", wsRpcPkg, ".MethodDesc{")
	for i, method := range service.Method {
		if method.GetServerStreaming() || method.GetClientStreaming() {
			continue
		}
		ws.P("{")
		ws.P("MethodName: ", strconv.Quote(method.GetName()), ",")
		ws.P("Handler: ", handlerNames[i], ",")
		ws.P("},")
	}
	ws.P("},")
	ws.P("Streams: []", wsRpcPkg, ".StreamDesc{")
	for i, method := range service.Method {
		if !method.GetServerStreaming() && !method.GetClientStreaming() {
			continue
		}
		ws.P("{")
		ws.P("StreamName: ", strconv.Quote(method.GetName()), ",")
		ws.P("Handler: ", handlerNames[i], ",")
		if method.GetServerStreaming() {
			ws.P("ServerStreams: true,")
		}
		if method.GetClientStreaming() {
			ws.P("ClientStreams: true,")
		}
		ws.P("},")
	}
	ws.P("},")
	ws.P("Metadata: \"", file.GetName(), "\",")
	ws.P("}")
	ws.P()
}

// generateServerSignature returns the server-side signature for a method.
func (ws *wsRPC) generateServerSignature(servName string, method *pb.MethodDescriptorProto) string {
	origMethName := method.GetName()
	methName := generator.CamelCase(origMethName)

	var reqArgs []string
	ret := "error"
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		reqArgs = append(reqArgs, contextPkg+".Context")
		ret = "(*" + ws.typeName(method.GetOutputType()) + ", error)"
	}
	if !method.GetClientStreaming() {
		reqArgs = append(reqArgs, "*"+ws.typeName(method.GetInputType()))
	}
	if method.GetServerStreaming() || method.GetClientStreaming() {
		reqArgs = append(reqArgs, servName+"_"+generator.CamelCase(origMethName)+"Server")
	}

	return methName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

func (ws *wsRPC) generateServerMethod(servName, fullServName string, method *pb.MethodDescriptorProto) string {
	methName := generator.CamelCase(method.GetName())
	hname := fmt.Sprintf("_%s_%s_Handler", servName, methName)
	inType := ws.typeName(method.GetInputType())
	outType := ws.typeName(method.GetOutputType())

	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		ws.P("func ", hname, "(srv interface{}, ctx ", contextPkg, ".Context, dec func(interface{}) error, interceptor ", wsRpcPkg, ".UnaryServerInterceptor) (interface{}, error) {")
		ws.P("in := new(", inType, ")")
		ws.P("if err := dec(in); err != nil { return nil, err }")
		ws.P("if interceptor == nil { return srv.(", servName, "Server).", methName, "(ctx, in) }")
		ws.P("info := &", wsRpcPkg, ".UnaryServerInfo{")
		ws.P("Server: srv,")
		ws.P("FullMethod: ", strconv.Quote(fmt.Sprintf("/%s/%s", fullServName, methName)), ",")
		ws.P("}")
		ws.P("handler := func(ctx ", contextPkg, ".Context, req interface{}) (interface{}, error) {")
		ws.P("return srv.(", servName, "Server).", methName, "(ctx, req.(*", inType, "))")
		ws.P("}")
		ws.P("return interceptor(ctx, in, info, handler)")
		ws.P("}")
		ws.P()
		return hname
	}
	streamType := unexport(servName) + methName + "Server"
	ws.P("func ", hname, "(srv interface{}, stream ", wsRpcPkg, ".ServerStream) error {")
	if !method.GetClientStreaming() {
		ws.P("m := new(", inType, ")")
		ws.P("if err := stream.RecvMsg(m); err != nil { return err }")
		ws.P("return srv.(", servName, "Server).", methName, "(m, &", streamType, "{stream})")
	} else {
		ws.P("return srv.(", servName, "Server).", methName, "(&", streamType, "{stream})")
	}
	ws.P("}")
	ws.P()

	genSend := method.GetServerStreaming()
	genSendAndClose := !method.GetServerStreaming()
	genRecv := method.GetClientStreaming()

	// Stream auxiliary types and methods.
	ws.P("type ", servName, "_", methName, "Server interface {")
	if genSend {
		ws.P("Send(*", outType, ") error")
	}
	if genSendAndClose {
		ws.P("SendAndClose(*", outType, ") error")
	}
	if genRecv {
		ws.P("Recv() (*", inType, ", error)")
	}
	ws.P(wsRpcPkg, ".ServerStream")
	ws.P("}")
	ws.P()

	ws.P("type ", streamType, " struct {")
	ws.P(wsRpcPkg, ".ServerStream")
	ws.P("}")
	ws.P()

	if genSend {
		ws.P("func (x *", streamType, ") Send(m *", outType, ") error {")
		ws.P("return x.ServerStream.SendMsg(m)")
		ws.P("}")
		ws.P()
	}
	if genSendAndClose {
		ws.P("func (x *", streamType, ") SendAndClose(m *", outType, ") error {")
		ws.P("return x.ServerStream.SendMsg(m)")
		ws.P("}")
		ws.P()
	}
	if genRecv {
		ws.P("func (x *", streamType, ") Recv() (*", inType, ", error) {")
		ws.P("m := new(", inType, ")")
		ws.P("if err := x.ServerStream.RecvMsg(m); err != nil { return nil, err }")
		ws.P("return m, nil")
		ws.P("}")
		ws.P()
	}

	return hname
}
