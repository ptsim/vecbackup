// protoc --go_out=. --go_opt=paths=source_relative *.proto
//

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0-devel
// 	protoc        v3.14.0
// source: formats.proto

package vecbackup

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type FileType int32

const (
	FileType_REGULAR_FILE FileType = 0
	FileType_DIRECTORY    FileType = 1
	FileType_SYMLINK      FileType = 2
)

// Enum value maps for FileType.
var (
	FileType_name = map[int32]string{
		0: "REGULAR_FILE",
		1: "DIRECTORY",
		2: "SYMLINK",
	}
	FileType_value = map[string]int32{
		"REGULAR_FILE": 0,
		"DIRECTORY":    1,
		"SYMLINK":      2,
	}
)

func (x FileType) Enum() *FileType {
	p := new(FileType)
	*p = x
	return p
}

func (x FileType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (FileType) Descriptor() protoreflect.EnumDescriptor {
	return file_formats_proto_enumTypes[0].Descriptor()
}

func (FileType) Type() protoreflect.EnumType {
	return &file_formats_proto_enumTypes[0]
}

func (x FileType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use FileType.Descriptor instead.
func (FileType) EnumDescriptor() ([]byte, []int) {
	return file_formats_proto_rawDescGZIP(), []int{0}
}

type EncType int32

const (
	EncType_NO_ENCRYPTION EncType = 0
	EncType_SYMMETRIC     EncType = 1
)

// Enum value maps for EncType.
var (
	EncType_name = map[int32]string{
		0: "NO_ENCRYPTION",
		1: "SYMMETRIC",
	}
	EncType_value = map[string]int32{
		"NO_ENCRYPTION": 0,
		"SYMMETRIC":     1,
	}
)

func (x EncType) Enum() *EncType {
	p := new(EncType)
	*p = x
	return p
}

func (x EncType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (EncType) Descriptor() protoreflect.EnumDescriptor {
	return file_formats_proto_enumTypes[1].Descriptor()
}

func (EncType) Type() protoreflect.EnumType {
	return &file_formats_proto_enumTypes[1]
}

func (x EncType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use EncType.Descriptor instead.
func (EncType) EnumDescriptor() ([]byte, []int) {
	return file_formats_proto_rawDescGZIP(), []int{1}
}

type CompressionType int32

const (
	CompressionType_NO_COMPRESSION CompressionType = 0
	CompressionType_ZLIB           CompressionType = 1
)

// Enum value maps for CompressionType.
var (
	CompressionType_name = map[int32]string{
		0: "NO_COMPRESSION",
		1: "ZLIB",
	}
	CompressionType_value = map[string]int32{
		"NO_COMPRESSION": 0,
		"ZLIB":           1,
	}
)

func (x CompressionType) Enum() *CompressionType {
	p := new(CompressionType)
	*p = x
	return p
}

func (x CompressionType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (CompressionType) Descriptor() protoreflect.EnumDescriptor {
	return file_formats_proto_enumTypes[2].Descriptor()
}

func (CompressionType) Type() protoreflect.EnumType {
	return &file_formats_proto_enumTypes[2]
}

func (x CompressionType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use CompressionType.Descriptor instead.
func (CompressionType) EnumDescriptor() ([]byte, []int) {
	return file_formats_proto_rawDescGZIP(), []int{2}
}

type CompressionMode int32

const (
	CompressionMode_AUTO CompressionMode = 0
	CompressionMode_SLOW CompressionMode = 1
	CompressionMode_NO   CompressionMode = 2
	CompressionMode_YES  CompressionMode = 3
)

// Enum value maps for CompressionMode.
var (
	CompressionMode_name = map[int32]string{
		0: "AUTO",
		1: "SLOW",
		2: "NO",
		3: "YES",
	}
	CompressionMode_value = map[string]int32{
		"AUTO": 0,
		"SLOW": 1,
		"NO":   2,
		"YES":  3,
	}
)

func (x CompressionMode) Enum() *CompressionMode {
	p := new(CompressionMode)
	*p = x
	return p
}

func (x CompressionMode) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (CompressionMode) Descriptor() protoreflect.EnumDescriptor {
	return file_formats_proto_enumTypes[3].Descriptor()
}

func (CompressionMode) Type() protoreflect.EnumType {
	return &file_formats_proto_enumTypes[3]
}

func (x CompressionMode) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use CompressionMode.Descriptor instead.
func (CompressionMode) EnumDescriptor() ([]byte, []int) {
	return file_formats_proto_rawDescGZIP(), []int{3}
}

type NodeDataProto struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name         string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Type         FileType               `protobuf:"varint,2,opt,name=type,proto3,enum=FileType" json:"type,omitempty"`
	Size         int64                  `protobuf:"varint,3,opt,name=size,proto3" json:"size,omitempty"`
	ModTime      *timestamppb.Timestamp `protobuf:"bytes,4,opt,name=mod_time,json=modTime,proto3" json:"mod_time,omitempty"`
	Perm         int32                  `protobuf:"varint,5,opt,name=perm,proto3" json:"perm,omitempty"`
	FileChecksum []byte                 `protobuf:"bytes,6,opt,name=FileChecksum,proto3" json:"FileChecksum,omitempty"`
	Target       string                 `protobuf:"bytes,7,opt,name=target,proto3" json:"target,omitempty"`
	Sizes        []int32                `protobuf:"varint,8,rep,packed,name=Sizes,proto3" json:"Sizes,omitempty"`
	Chunks       [][]byte               `protobuf:"bytes,9,rep,name=Chunks,proto3" json:"Chunks,omitempty"`
}

func (x *NodeDataProto) Reset() {
	*x = NodeDataProto{}
	if protoimpl.UnsafeEnabled {
		mi := &file_formats_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NodeDataProto) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NodeDataProto) ProtoMessage() {}

func (x *NodeDataProto) ProtoReflect() protoreflect.Message {
	mi := &file_formats_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NodeDataProto.ProtoReflect.Descriptor instead.
func (*NodeDataProto) Descriptor() ([]byte, []int) {
	return file_formats_proto_rawDescGZIP(), []int{0}
}

func (x *NodeDataProto) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *NodeDataProto) GetType() FileType {
	if x != nil {
		return x.Type
	}
	return FileType_REGULAR_FILE
}

func (x *NodeDataProto) GetSize() int64 {
	if x != nil {
		return x.Size
	}
	return 0
}

func (x *NodeDataProto) GetModTime() *timestamppb.Timestamp {
	if x != nil {
		return x.ModTime
	}
	return nil
}

func (x *NodeDataProto) GetPerm() int32 {
	if x != nil {
		return x.Perm
	}
	return 0
}

func (x *NodeDataProto) GetFileChecksum() []byte {
	if x != nil {
		return x.FileChecksum
	}
	return nil
}

func (x *NodeDataProto) GetTarget() string {
	if x != nil {
		return x.Target
	}
	return ""
}

func (x *NodeDataProto) GetSizes() []int32 {
	if x != nil {
		return x.Sizes
	}
	return nil
}

func (x *NodeDataProto) GetChunks() [][]byte {
	if x != nil {
		return x.Chunks
	}
	return nil
}

type VersionProto struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Version int32 `protobuf:"varint,1,opt,name=version,proto3" json:"version,omitempty"`
}

func (x *VersionProto) Reset() {
	*x = VersionProto{}
	if protoimpl.UnsafeEnabled {
		mi := &file_formats_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VersionProto) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VersionProto) ProtoMessage() {}

func (x *VersionProto) ProtoReflect() protoreflect.Message {
	mi := &file_formats_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VersionProto.ProtoReflect.Descriptor instead.
func (*VersionProto) Descriptor() ([]byte, []int) {
	return file_formats_proto_rawDescGZIP(), []int{1}
}

func (x *VersionProto) GetVersion() int32 {
	if x != nil {
		return x.Version
	}
	return 0
}

type ConfigProto struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ChunkSize     int32           `protobuf:"varint,1,opt,name=ChunkSize,proto3" json:"ChunkSize,omitempty"`
	EncryptionKey []byte          `protobuf:"bytes,2,opt,name=EncryptionKey,proto3" json:"EncryptionKey,omitempty"`
	FPSecret      []byte          `protobuf:"bytes,3,opt,name=FPSecret,proto3" json:"FPSecret,omitempty"`
	Compress      CompressionMode `protobuf:"varint,4,opt,name=Compress,proto3,enum=CompressionMode" json:"Compress,omitempty"`
}

func (x *ConfigProto) Reset() {
	*x = ConfigProto{}
	if protoimpl.UnsafeEnabled {
		mi := &file_formats_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ConfigProto) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConfigProto) ProtoMessage() {}

func (x *ConfigProto) ProtoReflect() protoreflect.Message {
	mi := &file_formats_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConfigProto.ProtoReflect.Descriptor instead.
func (*ConfigProto) Descriptor() ([]byte, []int) {
	return file_formats_proto_rawDescGZIP(), []int{2}
}

func (x *ConfigProto) GetChunkSize() int32 {
	if x != nil {
		return x.ChunkSize
	}
	return 0
}

func (x *ConfigProto) GetEncryptionKey() []byte {
	if x != nil {
		return x.EncryptionKey
	}
	return nil
}

func (x *ConfigProto) GetFPSecret() []byte {
	if x != nil {
		return x.FPSecret
	}
	return nil
}

func (x *ConfigProto) GetCompress() CompressionMode {
	if x != nil {
		return x.Compress
	}
	return CompressionMode_AUTO
}

type EncConfigProto struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Version    int32   `protobuf:"varint,1,opt,name=Version,proto3" json:"Version,omitempty"`
	Type       EncType `protobuf:"varint,2,opt,name=Type,proto3,enum=EncType" json:"Type,omitempty"`
	Iterations int64   `protobuf:"varint,3,opt,name=Iterations,proto3" json:"Iterations,omitempty"`
	Salt       []byte  `protobuf:"bytes,4,opt,name=Salt,proto3" json:"Salt,omitempty"`
	Config     []byte  `protobuf:"bytes,5,opt,name=Config,proto3" json:"Config,omitempty"`
}

func (x *EncConfigProto) Reset() {
	*x = EncConfigProto{}
	if protoimpl.UnsafeEnabled {
		mi := &file_formats_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EncConfigProto) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EncConfigProto) ProtoMessage() {}

func (x *EncConfigProto) ProtoReflect() protoreflect.Message {
	mi := &file_formats_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EncConfigProto.ProtoReflect.Descriptor instead.
func (*EncConfigProto) Descriptor() ([]byte, []int) {
	return file_formats_proto_rawDescGZIP(), []int{3}
}

func (x *EncConfigProto) GetVersion() int32 {
	if x != nil {
		return x.Version
	}
	return 0
}

func (x *EncConfigProto) GetType() EncType {
	if x != nil {
		return x.Type
	}
	return EncType_NO_ENCRYPTION
}

func (x *EncConfigProto) GetIterations() int64 {
	if x != nil {
		return x.Iterations
	}
	return 0
}

func (x *EncConfigProto) GetSalt() []byte {
	if x != nil {
		return x.Salt
	}
	return nil
}

func (x *EncConfigProto) GetConfig() []byte {
	if x != nil {
		return x.Config
	}
	return nil
}

var File_formats_proto protoreflect.FileDescriptor

var file_formats_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x66, 0x6f, 0x72, 0x6d, 0x61, 0x74, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0x8b, 0x02, 0x0a, 0x0d, 0x4e, 0x6f, 0x64, 0x65, 0x44, 0x61, 0x74, 0x61, 0x50, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1d, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0e, 0x32, 0x09, 0x2e, 0x46, 0x69, 0x6c, 0x65, 0x54, 0x79, 0x70, 0x65, 0x52,
	0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x12, 0x35, 0x0a, 0x08, 0x6d, 0x6f, 0x64,
	0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x07, 0x6d, 0x6f, 0x64, 0x54, 0x69, 0x6d, 0x65,
	0x12, 0x12, 0x0a, 0x04, 0x70, 0x65, 0x72, 0x6d, 0x18, 0x05, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04,
	0x70, 0x65, 0x72, 0x6d, 0x12, 0x22, 0x0a, 0x0c, 0x46, 0x69, 0x6c, 0x65, 0x43, 0x68, 0x65, 0x63,
	0x6b, 0x73, 0x75, 0x6d, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0c, 0x46, 0x69, 0x6c, 0x65,
	0x43, 0x68, 0x65, 0x63, 0x6b, 0x73, 0x75, 0x6d, 0x12, 0x16, 0x0a, 0x06, 0x74, 0x61, 0x72, 0x67,
	0x65, 0x74, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x74, 0x61, 0x72, 0x67, 0x65, 0x74,
	0x12, 0x14, 0x0a, 0x05, 0x53, 0x69, 0x7a, 0x65, 0x73, 0x18, 0x08, 0x20, 0x03, 0x28, 0x05, 0x52,
	0x05, 0x53, 0x69, 0x7a, 0x65, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x43, 0x68, 0x75, 0x6e, 0x6b, 0x73,
	0x18, 0x09, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x06, 0x43, 0x68, 0x75, 0x6e, 0x6b, 0x73, 0x22, 0x28,
	0x0a, 0x0c, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x18,
	0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52,
	0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x22, 0x9b, 0x01, 0x0a, 0x0b, 0x43, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1c, 0x0a, 0x09, 0x43, 0x68, 0x75, 0x6e,
	0x6b, 0x53, 0x69, 0x7a, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x09, 0x43, 0x68, 0x75,
	0x6e, 0x6b, 0x53, 0x69, 0x7a, 0x65, 0x12, 0x24, 0x0a, 0x0d, 0x45, 0x6e, 0x63, 0x72, 0x79, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x4b, 0x65, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0d, 0x45,
	0x6e, 0x63, 0x72, 0x79, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x4b, 0x65, 0x79, 0x12, 0x1a, 0x0a, 0x08,
	0x46, 0x50, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x08,
	0x46, 0x50, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x12, 0x2c, 0x0a, 0x08, 0x43, 0x6f, 0x6d, 0x70,
	0x72, 0x65, 0x73, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x10, 0x2e, 0x43, 0x6f, 0x6d,
	0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x4d, 0x6f, 0x64, 0x65, 0x52, 0x08, 0x43, 0x6f,
	0x6d, 0x70, 0x72, 0x65, 0x73, 0x73, 0x22, 0x94, 0x01, 0x0a, 0x0e, 0x45, 0x6e, 0x63, 0x43, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x18, 0x0a, 0x07, 0x56, 0x65, 0x72,
	0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x07, 0x56, 0x65, 0x72, 0x73,
	0x69, 0x6f, 0x6e, 0x12, 0x1c, 0x0a, 0x04, 0x54, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0e, 0x32, 0x08, 0x2e, 0x45, 0x6e, 0x63, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x54, 0x79, 0x70,
	0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x49, 0x74, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0a, 0x49, 0x74, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x12, 0x12, 0x0a, 0x04, 0x53, 0x61, 0x6c, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52,
	0x04, 0x53, 0x61, 0x6c, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x06, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2a, 0x38, 0x0a,
	0x08, 0x46, 0x69, 0x6c, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12, 0x10, 0x0a, 0x0c, 0x52, 0x45, 0x47,
	0x55, 0x4c, 0x41, 0x52, 0x5f, 0x46, 0x49, 0x4c, 0x45, 0x10, 0x00, 0x12, 0x0d, 0x0a, 0x09, 0x44,
	0x49, 0x52, 0x45, 0x43, 0x54, 0x4f, 0x52, 0x59, 0x10, 0x01, 0x12, 0x0b, 0x0a, 0x07, 0x53, 0x59,
	0x4d, 0x4c, 0x49, 0x4e, 0x4b, 0x10, 0x02, 0x2a, 0x2b, 0x0a, 0x07, 0x45, 0x6e, 0x63, 0x54, 0x79,
	0x70, 0x65, 0x12, 0x11, 0x0a, 0x0d, 0x4e, 0x4f, 0x5f, 0x45, 0x4e, 0x43, 0x52, 0x59, 0x50, 0x54,
	0x49, 0x4f, 0x4e, 0x10, 0x00, 0x12, 0x0d, 0x0a, 0x09, 0x53, 0x59, 0x4d, 0x4d, 0x45, 0x54, 0x52,
	0x49, 0x43, 0x10, 0x01, 0x2a, 0x2f, 0x0a, 0x0f, 0x43, 0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73, 0x73,
	0x69, 0x6f, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x12, 0x12, 0x0a, 0x0e, 0x4e, 0x4f, 0x5f, 0x43, 0x4f,
	0x4d, 0x50, 0x52, 0x45, 0x53, 0x53, 0x49, 0x4f, 0x4e, 0x10, 0x00, 0x12, 0x08, 0x0a, 0x04, 0x5a,
	0x4c, 0x49, 0x42, 0x10, 0x01, 0x2a, 0x36, 0x0a, 0x0f, 0x43, 0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73,
	0x73, 0x69, 0x6f, 0x6e, 0x4d, 0x6f, 0x64, 0x65, 0x12, 0x08, 0x0a, 0x04, 0x41, 0x55, 0x54, 0x4f,
	0x10, 0x00, 0x12, 0x08, 0x0a, 0x04, 0x53, 0x4c, 0x4f, 0x57, 0x10, 0x01, 0x12, 0x06, 0x0a, 0x02,
	0x4e, 0x4f, 0x10, 0x02, 0x12, 0x07, 0x0a, 0x03, 0x59, 0x45, 0x53, 0x10, 0x03, 0x42, 0x2f, 0x5a,
	0x2d, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x74, 0x73, 0x69,
	0x6d, 0x2f, 0x76, 0x65, 0x63, 0x62, 0x61, 0x63, 0x6b, 0x75, 0x70, 0x2f, 0x69, 0x6e, 0x74, 0x65,
	0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x76, 0x65, 0x63, 0x62, 0x61, 0x63, 0x6b, 0x75, 0x70, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_formats_proto_rawDescOnce sync.Once
	file_formats_proto_rawDescData = file_formats_proto_rawDesc
)

func file_formats_proto_rawDescGZIP() []byte {
	file_formats_proto_rawDescOnce.Do(func() {
		file_formats_proto_rawDescData = protoimpl.X.CompressGZIP(file_formats_proto_rawDescData)
	})
	return file_formats_proto_rawDescData
}

var file_formats_proto_enumTypes = make([]protoimpl.EnumInfo, 4)
var file_formats_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_formats_proto_goTypes = []interface{}{
	(FileType)(0),                 // 0: FileType
	(EncType)(0),                  // 1: EncType
	(CompressionType)(0),          // 2: CompressionType
	(CompressionMode)(0),          // 3: CompressionMode
	(*NodeDataProto)(nil),         // 4: NodeDataProto
	(*VersionProto)(nil),          // 5: VersionProto
	(*ConfigProto)(nil),           // 6: ConfigProto
	(*EncConfigProto)(nil),        // 7: EncConfigProto
	(*timestamppb.Timestamp)(nil), // 8: google.protobuf.Timestamp
}
var file_formats_proto_depIdxs = []int32{
	0, // 0: NodeDataProto.type:type_name -> FileType
	8, // 1: NodeDataProto.mod_time:type_name -> google.protobuf.Timestamp
	3, // 2: ConfigProto.Compress:type_name -> CompressionMode
	1, // 3: EncConfigProto.Type:type_name -> EncType
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_formats_proto_init() }
func file_formats_proto_init() {
	if File_formats_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_formats_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NodeDataProto); i {
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
		file_formats_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*VersionProto); i {
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
		file_formats_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ConfigProto); i {
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
		file_formats_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EncConfigProto); i {
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
			RawDescriptor: file_formats_proto_rawDesc,
			NumEnums:      4,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_formats_proto_goTypes,
		DependencyIndexes: file_formats_proto_depIdxs,
		EnumInfos:         file_formats_proto_enumTypes,
		MessageInfos:      file_formats_proto_msgTypes,
	}.Build()
	File_formats_proto = out.File
	file_formats_proto_rawDesc = nil
	file_formats_proto_goTypes = nil
	file_formats_proto_depIdxs = nil
}
