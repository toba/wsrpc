/*
 * Copyright 2014 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package wsrpc

import (
	"math"
	"sync"

	"github.com/golang/protobuf/proto"
)

type (
	// protoCodec is a grpc.Codec implementation with protobuf. It is a direct
	// copy of grpc.protoCodec. We would use it directly but it's not exported.
	protoCodec struct{}

	cachedProtoBuffer struct {
		lastMarshaledSize uint32
		proto.Buffer
	}
)

var (
	protoBufferPool = &sync.Pool{
		New: func() interface{} {
			return &cachedProtoBuffer{
				Buffer:            proto.Buffer{},
				lastMarshaledSize: 16,
			}
		},
	}
)

func capToMaxInt32(val int) uint32 {
	if val > math.MaxInt32 {
		return uint32(math.MaxInt32)
	}
	return uint32(val)
}

func (p protoCodec) marshal(v interface{}, cb *cachedProtoBuffer) ([]byte, error) {
	protoMsg := v.(proto.Message)
	newSlice := make([]byte, 0, cb.lastMarshaledSize)

	cb.SetBuf(newSlice)
	cb.Reset()
	if err := cb.Marshal(protoMsg); err != nil {
		return nil, err
	}
	out := cb.Bytes()
	cb.lastMarshaledSize = capToMaxInt32(len(out))
	return out, nil
}

func (p protoCodec) Marshal(v interface{}) ([]byte, error) {
	cb := protoBufferPool.Get().(*cachedProtoBuffer)
	out, err := p.marshal(v, cb)

	// put back buffer and lose the ref to the slice
	cb.SetBuf(nil)
	protoBufferPool.Put(cb)
	return out, err
}

func (p protoCodec) Unmarshal(data []byte, v interface{}) error {
	cb := protoBufferPool.Get().(*cachedProtoBuffer)
	cb.SetBuf(data)
	v.(proto.Message).Reset()
	err := cb.Unmarshal(v.(proto.Message))
	cb.SetBuf(nil)
	protoBufferPool.Put(cb)
	return err
}

func (protoCodec) String() string {
	return "proto"
}
