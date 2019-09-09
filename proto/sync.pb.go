// Code generated by protoc-gen-go. DO NOT EDIT.
// source: sync.proto

package kv

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type OperationType int32

const (
	OperationType_UPDATE OperationType = 0
	OperationType_DELETE OperationType = 1
)

var OperationType_name = map[int32]string{
	0: "UPDATE",
	1: "DELETE",
}

var OperationType_value = map[string]int32{
	"UPDATE": 0,
	"DELETE": 1,
}

func (x OperationType) String() string {
	return proto.EnumName(OperationType_name, int32(x))
}

func (OperationType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_5273b98214de8075, []int{0}
}

type SyncType int32

const (
	SyncType_SYN SyncType = 0
	SyncType_ACK SyncType = 1
)

var SyncType_name = map[int32]string{
	0: "SYN",
	1: "ACK",
}

var SyncType_value = map[string]int32{
	"SYN": 0,
	"ACK": 1,
}

func (x SyncType) String() string {
	return proto.EnumName(SyncType_name, int32(x))
}

func (SyncType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_5273b98214de8075, []int{1}
}

type IdentifyMessage struct {
	StartTime            int64    `protobuf:"varint,1,opt,name=start_time,json=startTime,proto3" json:"start_time,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *IdentifyMessage) Reset()         { *m = IdentifyMessage{} }
func (m *IdentifyMessage) String() string { return proto.CompactTextString(m) }
func (*IdentifyMessage) ProtoMessage()    {}
func (*IdentifyMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_5273b98214de8075, []int{0}
}

func (m *IdentifyMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_IdentifyMessage.Unmarshal(m, b)
}
func (m *IdentifyMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_IdentifyMessage.Marshal(b, m, deterministic)
}
func (m *IdentifyMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_IdentifyMessage.Merge(m, src)
}
func (m *IdentifyMessage) XXX_Size() int {
	return xxx_messageInfo_IdentifyMessage.Size(m)
}
func (m *IdentifyMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_IdentifyMessage.DiscardUnknown(m)
}

var xxx_messageInfo_IdentifyMessage proto.InternalMessageInfo

func (m *IdentifyMessage) GetStartTime() int64 {
	if m != nil {
		return m.StartTime
	}
	return 0
}

type SyncMessage struct {
	Key                  string        `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Value                []byte        `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
	Timestamp            int64         `protobuf:"varint,3,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	OperationType        OperationType `protobuf:"varint,4,opt,name=operation_type,json=operationType,proto3,enum=kv.OperationType" json:"operation_type,omitempty"`
	SyncType             SyncType      `protobuf:"varint,5,opt,name=sync_type,json=syncType,proto3,enum=kv.SyncType" json:"sync_type,omitempty"`
	Coverage             uint64        `protobuf:"varint,6,opt,name=coverage,proto3" json:"coverage,omitempty"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *SyncMessage) Reset()         { *m = SyncMessage{} }
func (m *SyncMessage) String() string { return proto.CompactTextString(m) }
func (*SyncMessage) ProtoMessage()    {}
func (*SyncMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_5273b98214de8075, []int{1}
}

func (m *SyncMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SyncMessage.Unmarshal(m, b)
}
func (m *SyncMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SyncMessage.Marshal(b, m, deterministic)
}
func (m *SyncMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SyncMessage.Merge(m, src)
}
func (m *SyncMessage) XXX_Size() int {
	return xxx_messageInfo_SyncMessage.Size(m)
}
func (m *SyncMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_SyncMessage.DiscardUnknown(m)
}

var xxx_messageInfo_SyncMessage proto.InternalMessageInfo

func (m *SyncMessage) GetKey() string {
	if m != nil {
		return m.Key
	}
	return ""
}

func (m *SyncMessage) GetValue() []byte {
	if m != nil {
		return m.Value
	}
	return nil
}

func (m *SyncMessage) GetTimestamp() int64 {
	if m != nil {
		return m.Timestamp
	}
	return 0
}

func (m *SyncMessage) GetOperationType() OperationType {
	if m != nil {
		return m.OperationType
	}
	return OperationType_UPDATE
}

func (m *SyncMessage) GetSyncType() SyncType {
	if m != nil {
		return m.SyncType
	}
	return SyncType_SYN
}

func (m *SyncMessage) GetCoverage() uint64 {
	if m != nil {
		return m.Coverage
	}
	return 0
}

func init() {
	proto.RegisterEnum("kv.OperationType", OperationType_name, OperationType_value)
	proto.RegisterEnum("kv.SyncType", SyncType_name, SyncType_value)
	proto.RegisterType((*IdentifyMessage)(nil), "kv.IdentifyMessage")
	proto.RegisterType((*SyncMessage)(nil), "kv.SyncMessage")
}

func init() { proto.RegisterFile("sync.proto", fileDescriptor_5273b98214de8075) }

var fileDescriptor_5273b98214de8075 = []byte{
	// 313 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x51, 0x4f, 0x4f, 0xfa, 0x40,
	0x10, 0x65, 0x29, 0xf0, 0x6b, 0xe7, 0xc7, 0x9f, 0x3a, 0x7a, 0x68, 0x08, 0x26, 0x0d, 0x17, 0x2b,
	0x31, 0x84, 0xa0, 0x07, 0xaf, 0x44, 0x7a, 0x30, 0xfe, 0x4d, 0xa9, 0x07, 0x4f, 0xa4, 0xe2, 0x48,
	0x2a, 0xd2, 0x36, 0xdd, 0xb5, 0xc9, 0x7e, 0x4f, 0x3f, 0x90, 0xd9, 0xc2, 0x12, 0x25, 0xde, 0xde,
	0xbc, 0x79, 0xef, 0xed, 0xcc, 0x2c, 0x00, 0x97, 0xc9, 0x62, 0x98, 0xe5, 0xa9, 0x48, 0xb1, 0xba,
	0x2a, 0xfa, 0x23, 0xe8, 0x5c, 0xbf, 0x52, 0x22, 0xe2, 0x37, 0x79, 0x47, 0x9c, 0x47, 0x4b, 0xc2,
	0x63, 0x00, 0x2e, 0xa2, 0x5c, 0xcc, 0x45, 0xbc, 0x26, 0x87, 0xb9, 0xcc, 0x33, 0x02, 0xab, 0x64,
	0xc2, 0x78, 0x4d, 0xfd, 0x2f, 0x06, 0xff, 0x67, 0x32, 0x59, 0x68, 0xb9, 0x0d, 0xc6, 0x8a, 0x64,
	0xa9, 0xb3, 0x02, 0x05, 0xf1, 0x08, 0xea, 0x45, 0xf4, 0xf1, 0x49, 0x4e, 0xd5, 0x65, 0x5e, 0x33,
	0xd8, 0x14, 0xd8, 0x03, 0x4b, 0x05, 0x72, 0x11, 0xad, 0x33, 0xc7, 0xd8, 0xa4, 0xee, 0x08, 0xbc,
	0x84, 0x76, 0x9a, 0x51, 0x1e, 0x89, 0x38, 0x4d, 0xe6, 0x42, 0x66, 0xe4, 0xd4, 0x5c, 0xe6, 0xb5,
	0xc7, 0x07, 0xc3, 0x55, 0x31, 0x7c, 0xd0, 0x9d, 0x50, 0x66, 0x14, 0xb4, 0xd2, 0x9f, 0x25, 0x9e,
	0x82, 0xa5, 0x76, 0xda, 0x98, 0xea, 0xa5, 0xa9, 0xa9, 0x4c, 0x6a, 0xc6, 0x52, 0x6f, 0xf2, 0x2d,
	0xc2, 0x2e, 0x98, 0x8b, 0xb4, 0xa0, 0x3c, 0x5a, 0x92, 0xd3, 0x70, 0x99, 0x57, 0x0b, 0x76, 0xf5,
	0xe0, 0x04, 0x5a, 0xbf, 0x9e, 0x41, 0x80, 0xc6, 0xd3, 0xe3, 0x74, 0x12, 0xfa, 0x76, 0x45, 0xe1,
	0xa9, 0x7f, 0xeb, 0x87, 0xbe, 0xcd, 0x06, 0x3d, 0x30, 0x75, 0x34, 0xfe, 0x03, 0x63, 0xf6, 0x7c,
	0x6f, 0x57, 0x14, 0x98, 0x5c, 0xdd, 0xd8, 0x6c, 0xfc, 0x0e, 0x35, 0xd5, 0xc5, 0x0b, 0x30, 0xf5,
	0x5d, 0xf1, 0x50, 0x8d, 0xb3, 0x77, 0xe5, 0xee, 0x5f, 0x24, 0x9e, 0x6d, 0xdd, 0x1d, 0xbd, 0x80,
	0x56, 0xef, 0x13, 0x1e, 0x1b, 0xb1, 0x97, 0x46, 0xf9, 0x8d, 0xe7, 0xdf, 0x01, 0x00, 0x00, 0xff,
	0xff, 0xf3, 0x78, 0x41, 0xff, 0xd4, 0x01, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// SyncClient is the client API for Sync service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type SyncClient interface {
	Identify(ctx context.Context, in *IdentifyMessage, opts ...grpc.CallOption) (*IdentifyMessage, error)
	Sync(ctx context.Context, opts ...grpc.CallOption) (Sync_SyncClient, error)
}

type syncClient struct {
	cc *grpc.ClientConn
}

func NewSyncClient(cc *grpc.ClientConn) SyncClient {
	return &syncClient{cc}
}

func (c *syncClient) Identify(ctx context.Context, in *IdentifyMessage, opts ...grpc.CallOption) (*IdentifyMessage, error) {
	out := new(IdentifyMessage)
	err := c.cc.Invoke(ctx, "/kv.Sync/Identify", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *syncClient) Sync(ctx context.Context, opts ...grpc.CallOption) (Sync_SyncClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Sync_serviceDesc.Streams[0], "/kv.Sync/Sync", opts...)
	if err != nil {
		return nil, err
	}
	x := &syncSyncClient{stream}
	return x, nil
}

type Sync_SyncClient interface {
	Send(*SyncMessage) error
	Recv() (*SyncMessage, error)
	grpc.ClientStream
}

type syncSyncClient struct {
	grpc.ClientStream
}

func (x *syncSyncClient) Send(m *SyncMessage) error {
	return x.ClientStream.SendMsg(m)
}

func (x *syncSyncClient) Recv() (*SyncMessage, error) {
	m := new(SyncMessage)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// SyncServer is the server API for Sync service.
type SyncServer interface {
	Identify(context.Context, *IdentifyMessage) (*IdentifyMessage, error)
	Sync(Sync_SyncServer) error
}

// UnimplementedSyncServer can be embedded to have forward compatible implementations.
type UnimplementedSyncServer struct {
}

func (*UnimplementedSyncServer) Identify(ctx context.Context, req *IdentifyMessage) (*IdentifyMessage, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Identify not implemented")
}
func (*UnimplementedSyncServer) Sync(srv Sync_SyncServer) error {
	return status.Errorf(codes.Unimplemented, "method Sync not implemented")
}

func RegisterSyncServer(s *grpc.Server, srv SyncServer) {
	s.RegisterService(&_Sync_serviceDesc, srv)
}

func _Sync_Identify_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(IdentifyMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncServer).Identify(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/kv.Sync/Identify",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncServer).Identify(ctx, req.(*IdentifyMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _Sync_Sync_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(SyncServer).Sync(&syncSyncServer{stream})
}

type Sync_SyncServer interface {
	Send(*SyncMessage) error
	Recv() (*SyncMessage, error)
	grpc.ServerStream
}

type syncSyncServer struct {
	grpc.ServerStream
}

func (x *syncSyncServer) Send(m *SyncMessage) error {
	return x.ServerStream.SendMsg(m)
}

func (x *syncSyncServer) Recv() (*SyncMessage, error) {
	m := new(SyncMessage)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _Sync_serviceDesc = grpc.ServiceDesc{
	ServiceName: "kv.Sync",
	HandlerType: (*SyncServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Identify",
			Handler:    _Sync_Identify_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Sync",
			Handler:       _Sync_Sync_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "sync.proto",
}