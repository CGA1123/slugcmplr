// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        v3.17.0
// source: proto/ping/ping.proto

package ping

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type BoomRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *BoomRequest) Reset() {
	*x = BoomRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_ping_ping_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BoomRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BoomRequest) ProtoMessage() {}

func (x *BoomRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_ping_ping_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BoomRequest.ProtoReflect.Descriptor instead.
func (*BoomRequest) Descriptor() ([]byte, []int) {
	return file_proto_ping_ping_proto_rawDescGZIP(), []int{0}
}

type BoomResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *BoomResponse) Reset() {
	*x = BoomResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_ping_ping_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BoomResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BoomResponse) ProtoMessage() {}

func (x *BoomResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_ping_ping_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BoomResponse.ProtoReflect.Descriptor instead.
func (*BoomResponse) Descriptor() ([]byte, []int) {
	return file_proto_ping_ping_proto_rawDescGZIP(), []int{1}
}

type DatabaseHealthRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *DatabaseHealthRequest) Reset() {
	*x = DatabaseHealthRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_ping_ping_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DatabaseHealthRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DatabaseHealthRequest) ProtoMessage() {}

func (x *DatabaseHealthRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_ping_ping_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DatabaseHealthRequest.ProtoReflect.Descriptor instead.
func (*DatabaseHealthRequest) Descriptor() ([]byte, []int) {
	return file_proto_ping_ping_proto_rawDescGZIP(), []int{2}
}

type DatabaseHealthResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *DatabaseHealthResponse) Reset() {
	*x = DatabaseHealthResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_ping_ping_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DatabaseHealthResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DatabaseHealthResponse) ProtoMessage() {}

func (x *DatabaseHealthResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_ping_ping_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DatabaseHealthResponse.ProtoReflect.Descriptor instead.
func (*DatabaseHealthResponse) Descriptor() ([]byte, []int) {
	return file_proto_ping_ping_proto_rawDescGZIP(), []int{3}
}

type EchoRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Msg string `protobuf:"bytes,1,opt,name=msg,proto3" json:"msg,omitempty"`
}

func (x *EchoRequest) Reset() {
	*x = EchoRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_ping_ping_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EchoRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EchoRequest) ProtoMessage() {}

func (x *EchoRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_ping_ping_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EchoRequest.ProtoReflect.Descriptor instead.
func (*EchoRequest) Descriptor() ([]byte, []int) {
	return file_proto_ping_ping_proto_rawDescGZIP(), []int{4}
}

func (x *EchoRequest) GetMsg() string {
	if x != nil {
		return x.Msg
	}
	return ""
}

type EchoResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Msg string `protobuf:"bytes,1,opt,name=msg,proto3" json:"msg,omitempty"`
}

func (x *EchoResponse) Reset() {
	*x = EchoResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_ping_ping_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EchoResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EchoResponse) ProtoMessage() {}

func (x *EchoResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_ping_ping_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EchoResponse.ProtoReflect.Descriptor instead.
func (*EchoResponse) Descriptor() ([]byte, []int) {
	return file_proto_ping_ping_proto_rawDescGZIP(), []int{5}
}

func (x *EchoResponse) GetMsg() string {
	if x != nil {
		return x.Msg
	}
	return ""
}

type QueueRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Msg string `protobuf:"bytes,1,opt,name=msg,proto3" json:"msg,omitempty"`
}

func (x *QueueRequest) Reset() {
	*x = QueueRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_ping_ping_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *QueueRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueueRequest) ProtoMessage() {}

func (x *QueueRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_ping_ping_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueueRequest.ProtoReflect.Descriptor instead.
func (*QueueRequest) Descriptor() ([]byte, []int) {
	return file_proto_ping_ping_proto_rawDescGZIP(), []int{6}
}

func (x *QueueRequest) GetMsg() string {
	if x != nil {
		return x.Msg
	}
	return ""
}

type QueueResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Jid string `protobuf:"bytes,1,opt,name=jid,proto3" json:"jid,omitempty"`
}

func (x *QueueResponse) Reset() {
	*x = QueueResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_ping_ping_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *QueueResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueueResponse) ProtoMessage() {}

func (x *QueueResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_ping_ping_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueueResponse.ProtoReflect.Descriptor instead.
func (*QueueResponse) Descriptor() ([]byte, []int) {
	return file_proto_ping_ping_proto_rawDescGZIP(), []int{7}
}

func (x *QueueResponse) GetJid() string {
	if x != nil {
		return x.Jid
	}
	return ""
}

var File_proto_ping_ping_proto protoreflect.FileDescriptor

var file_proto_ping_ping_proto_rawDesc = []byte{
	0x0a, 0x15, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x69, 0x6e, 0x67, 0x2f, 0x70, 0x69, 0x6e,
	0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x04, 0x70, 0x69, 0x6e, 0x67, 0x22, 0x0d, 0x0a,
	0x0b, 0x42, 0x6f, 0x6f, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x0e, 0x0a, 0x0c,
	0x42, 0x6f, 0x6f, 0x6d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x17, 0x0a, 0x15,
	0x44, 0x61, 0x74, 0x61, 0x62, 0x61, 0x73, 0x65, 0x48, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x18, 0x0a, 0x16, 0x44, 0x61, 0x74, 0x61, 0x62, 0x61, 0x73,
	0x65, 0x48, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x1f, 0x0a, 0x0b, 0x45, 0x63, 0x68, 0x6f, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x10,
	0x0a, 0x03, 0x6d, 0x73, 0x67, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6d, 0x73, 0x67,
	0x22, 0x20, 0x0a, 0x0c, 0x45, 0x63, 0x68, 0x6f, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x10, 0x0a, 0x03, 0x6d, 0x73, 0x67, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6d,
	0x73, 0x67, 0x22, 0x20, 0x0a, 0x0c, 0x51, 0x75, 0x65, 0x75, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x6d, 0x73, 0x67, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x03, 0x6d, 0x73, 0x67, 0x22, 0x21, 0x0a, 0x0d, 0x51, 0x75, 0x65, 0x75, 0x65, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x6a, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x03, 0x6a, 0x69, 0x64, 0x32, 0xe3, 0x01, 0x0a, 0x04, 0x50, 0x69, 0x6e, 0x67,
	0x12, 0x2d, 0x0a, 0x04, 0x45, 0x63, 0x68, 0x6f, 0x12, 0x11, 0x2e, 0x70, 0x69, 0x6e, 0x67, 0x2e,
	0x45, 0x63, 0x68, 0x6f, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x12, 0x2e, 0x70, 0x69,
	0x6e, 0x67, 0x2e, 0x45, 0x63, 0x68, 0x6f, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x2d, 0x0a, 0x04, 0x42, 0x6f, 0x6f, 0x6d, 0x12, 0x11, 0x2e, 0x70, 0x69, 0x6e, 0x67, 0x2e, 0x42,
	0x6f, 0x6f, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x12, 0x2e, 0x70, 0x69, 0x6e,
	0x67, 0x2e, 0x42, 0x6f, 0x6f, 0x6d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x4b,
	0x0a, 0x0e, 0x44, 0x61, 0x74, 0x61, 0x62, 0x61, 0x73, 0x65, 0x48, 0x65, 0x61, 0x6c, 0x74, 0x68,
	0x12, 0x1b, 0x2e, 0x70, 0x69, 0x6e, 0x67, 0x2e, 0x44, 0x61, 0x74, 0x61, 0x62, 0x61, 0x73, 0x65,
	0x48, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1c, 0x2e,
	0x70, 0x69, 0x6e, 0x67, 0x2e, 0x44, 0x61, 0x74, 0x61, 0x62, 0x61, 0x73, 0x65, 0x48, 0x65, 0x61,
	0x6c, 0x74, 0x68, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x30, 0x0a, 0x05, 0x51,
	0x75, 0x65, 0x75, 0x65, 0x12, 0x12, 0x2e, 0x70, 0x69, 0x6e, 0x67, 0x2e, 0x51, 0x75, 0x65, 0x75,
	0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x13, 0x2e, 0x70, 0x69, 0x6e, 0x67, 0x2e,
	0x51, 0x75, 0x65, 0x75, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x29, 0x5a,
	0x27, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x67, 0x61, 0x31,
	0x31, 0x32, 0x33, 0x2f, 0x73, 0x6c, 0x75, 0x67, 0x63, 0x6d, 0x70, 0x6c, 0x72, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x69, 0x6e, 0x67, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_ping_ping_proto_rawDescOnce sync.Once
	file_proto_ping_ping_proto_rawDescData = file_proto_ping_ping_proto_rawDesc
)

func file_proto_ping_ping_proto_rawDescGZIP() []byte {
	file_proto_ping_ping_proto_rawDescOnce.Do(func() {
		file_proto_ping_ping_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_ping_ping_proto_rawDescData)
	})
	return file_proto_ping_ping_proto_rawDescData
}

var file_proto_ping_ping_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_proto_ping_ping_proto_goTypes = []interface{}{
	(*BoomRequest)(nil),            // 0: ping.BoomRequest
	(*BoomResponse)(nil),           // 1: ping.BoomResponse
	(*DatabaseHealthRequest)(nil),  // 2: ping.DatabaseHealthRequest
	(*DatabaseHealthResponse)(nil), // 3: ping.DatabaseHealthResponse
	(*EchoRequest)(nil),            // 4: ping.EchoRequest
	(*EchoResponse)(nil),           // 5: ping.EchoResponse
	(*QueueRequest)(nil),           // 6: ping.QueueRequest
	(*QueueResponse)(nil),          // 7: ping.QueueResponse
}
var file_proto_ping_ping_proto_depIdxs = []int32{
	4, // 0: ping.Ping.Echo:input_type -> ping.EchoRequest
	0, // 1: ping.Ping.Boom:input_type -> ping.BoomRequest
	2, // 2: ping.Ping.DatabaseHealth:input_type -> ping.DatabaseHealthRequest
	6, // 3: ping.Ping.Queue:input_type -> ping.QueueRequest
	5, // 4: ping.Ping.Echo:output_type -> ping.EchoResponse
	1, // 5: ping.Ping.Boom:output_type -> ping.BoomResponse
	3, // 6: ping.Ping.DatabaseHealth:output_type -> ping.DatabaseHealthResponse
	7, // 7: ping.Ping.Queue:output_type -> ping.QueueResponse
	4, // [4:8] is the sub-list for method output_type
	0, // [0:4] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_proto_ping_ping_proto_init() }
func file_proto_ping_ping_proto_init() {
	if File_proto_ping_ping_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_ping_ping_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BoomRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_ping_ping_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BoomResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_ping_ping_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DatabaseHealthRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_ping_ping_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DatabaseHealthResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_ping_ping_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EchoRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_ping_ping_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EchoResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_ping_ping_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*QueueRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_ping_ping_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*QueueResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_ping_ping_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_ping_ping_proto_goTypes,
		DependencyIndexes: file_proto_ping_ping_proto_depIdxs,
		MessageInfos:      file_proto_ping_ping_proto_msgTypes,
	}.Build()
	File_proto_ping_ping_proto = out.File
	file_proto_ping_ping_proto_rawDesc = nil
	file_proto_ping_ping_proto_goTypes = nil
	file_proto_ping_ping_proto_depIdxs = nil
}
