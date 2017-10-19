# wsrpc
A [protoc plugin](https://github.com/golang/protobuf/tree/master/protoc-gen-go) to generate Go server and Javascript client [Protocol Buffer](https://developers.google.com/protocol-buffers/) files to be used for service calls across WebSockets rather than [gRPC](https://grpc.io/).
(See [jnordberg/wsrpc](https://github.com/jnordberg/wsrpc) for a pure Javascript solution.)

The first release will generate a TypeScript client to avoid the need for type validations in libraries like [dcodeIO/protobuf.js](https://github.com/dcodeIO/ProtoBuf.js/). Which flavor of Javascript client to generate will be implemented as a secondary plugin per the `Plugin` interface in the [protoc-gen-go generator](https://github.com/golang/protobuf/blob/master/protoc-gen-go/generator/generator.go).

For project status, see the [issues and milestones](https://github.com/toba/wsrpc/issues).

