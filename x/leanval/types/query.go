package types

import (
	fmt "fmt"
	io "io"

	proto "github.com/cosmos/gogoproto/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	context "context"
)

const _ = proto.GoGoProtoPackageIsVersion3

type QueryBondedSetRequest struct {
	Period uint64 `protobuf:"varint,1,opt,name=period,proto3" json:"period,omitempty"`
}

func (m *QueryBondedSetRequest) GetPeriod() uint64 {
	if m == nil {
		return 0
	}
	return m.Period
}

func (m *QueryBondedSetRequest) Reset()         { *m = QueryBondedSetRequest{} }
func (m *QueryBondedSetRequest) String() string { return proto.CompactTextString(m) }
func (*QueryBondedSetRequest) ProtoMessage()    {}
func (m *QueryBondedSetRequest) XXX_Size() int  { return m.Size() }
func (m *QueryBondedSetRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *QueryBondedSetRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return m.Marshal()
}

func (m *QueryBondedSetRequest) Size() int {
	if m == nil || m.Period == 0 {
		return 0
	}
	n := 1 + sov(m.Period)
	return n
}

func (m *QueryBondedSetRequest) Marshal() ([]byte, error) {
	size := m.Size()
	dAtA := make([]byte, size)
	i := len(dAtA)
	if m.Period != 0 {
		i = encodeVarint(dAtA, i, m.Period)
		i--
		dAtA[i] = 0x8
	}
	return dAtA, nil
}

func (m *QueryBondedSetRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		if fieldNum == 1 {
			m.Period = 0
			for shift := uint(0); ; shift += 7 {
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Period |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		} else {
			return fmt.Errorf("leanval query: unexpected field %d", fieldNum)
		}
	}
	return nil
}

type BondedSetRow struct {
	Subject  []byte `json:"subject"`
	Weight   int64  `json:"weight"`
	HasProof bool   `json:"has_proof"`
}

type QueryBondedSetResponse struct {
	Rows []BondedSetRow `json:"rows"`
	raw  []byte
}

func (m *QueryBondedSetResponse) Reset()         { *m = QueryBondedSetResponse{} }
func (m *QueryBondedSetResponse) String() string { return proto.CompactTextString(m) }
func (*QueryBondedSetResponse) ProtoMessage()    {}
func (m *QueryBondedSetResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *QueryBondedSetResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return m.Marshal()
}

func (m *QueryBondedSetResponse) Marshal() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	body := m.raw
	if body == nil {
		// field 1: bytes json
		js, err := jsonRows(m.Rows)
		if err != nil {
			return nil, err
		}
		body = js
	}
	i := 1 + sov(uint64(len(body))) + len(body)
	dAtA := make([]byte, i)
	off := 0
	dAtA[off] = 0xa
	off++
	off = putVarint(dAtA, off, uint64(len(body)))
	copy(dAtA[off:], body)
	return dAtA, nil
}

func (m *QueryBondedSetResponse) Unmarshal(dAtA []byte) error {
	if len(dAtA) == 0 {
		return nil
	}
	if dAtA[0] != 0xa {
		return fmt.Errorf("leanval query: expected bytes field")
	}
	i := 1
	l, n := consumeVarint(dAtA[i:])
	i += n
	if i+int(l) > len(dAtA) {
		return io.ErrUnexpectedEOF
	}
	return unmarshalRows(dAtA[i:i+int(l)], m)
}

func jsonRows(rows []BondedSetRow) ([]byte, error) {
	return protoJSONRows(rows)
}

func protoJSONRows(rows []BondedSetRow) ([]byte, error) {
	return jsonMarshalRows(rows)
}

type QueryServer interface {
	BondedSet(context.Context, *QueryBondedSetRequest) (*QueryBondedSetResponse, error)
}

type QueryClient interface {
	BondedSet(ctx context.Context, in *QueryBondedSetRequest, opts ...grpc.CallOption) (*QueryBondedSetResponse, error)
}

type queryClient struct{ cc grpc.ClientConnInterface }

func NewQueryClient(cc grpc.ClientConnInterface) QueryClient {
	return &queryClient{cc}
}

func (c *queryClient) BondedSet(ctx context.Context, in *QueryBondedSetRequest, opts ...grpc.CallOption) (*QueryBondedSetResponse, error) {
	out := new(QueryBondedSetResponse)
	err := c.cc.Invoke(ctx, "/terp.leanval.v1.Query/BondedSet", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func RegisterQueryServer(s grpc.ServiceRegistrar, srv QueryServer) {
	s.RegisterService(&Query_ServiceDesc, srv)
}

func _Query_BondedSet_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryBondedSetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).BondedSet(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/terp.leanval.v1.Query/BondedSet"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).BondedSet(ctx, req.(*QueryBondedSetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var Query_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "terp.leanval.v1.Query",
	HandlerType: (*QueryServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "BondedSet", Handler: _Query_BondedSet_Handler},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "terp/leanval/v1/query.proto",
}

func encodeVarint(dAtA []byte, offset int, v uint64) int {
	offset -= sov(v)
	putVarint(dAtA, offset, v)
	return offset
}

func putVarint(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}

func consumeVarint(b []byte) (uint64, int) {
	var x uint64
	var s uint
	for i, c := range b {
		if c < 0x80 {
			if i > 9 || i == 9 && c > 1 {
				return 0, 0
			}
			return x | uint64(c)<<s, i + 1
		}
		x |= uint64(c&0x7f) << s
		s += 7
	}
	return 0, 0
}

func sov(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			return n
		}
	}
}

func _() {
	_ = codes.OK
	_ = status.Error
}
