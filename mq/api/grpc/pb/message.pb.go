// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package pb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type TaskMessage struct {
	TaskType   string `protobuf:"bytes,1,opt,name=task_type,json=taskType" json:"task_type,omitempty"`
	TaskBody   []byte `protobuf:"bytes,2,opt,name=task_body,json=taskBody,proto3" json:"task_body,omitempty"`
	CreateTime string `protobuf:"bytes,3,opt,name=create_time,json=createTime" json:"create_time,omitempty"`
	User       string `protobuf:"bytes,4,opt,name=user" json:"user,omitempty"`
}

func (m *TaskMessage) Reset()                    { *m = TaskMessage{} }
func (m *TaskMessage) String() string            { return proto.CompactTextString(m) }
func (*TaskMessage) ProtoMessage()               {}
func (*TaskMessage) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *TaskMessage) GetTaskType() string {
	if m != nil {
		return m.TaskType
	}
	return ""
}

func (m *TaskMessage) GetTaskBody() []byte {
	if m != nil {
		return m.TaskBody
	}
	return nil
}

func (m *TaskMessage) GetCreateTime() string {
	if m != nil {
		return m.CreateTime
	}
	return ""
}

func (m *TaskMessage) GetUser() string {
	if m != nil {
		return m.User
	}
	return ""
}

type EnqueueRequest struct {
	Topic   string       `protobuf:"bytes,1,opt,name=topic" json:"topic,omitempty"`
	Message *TaskMessage `protobuf:"bytes,2,opt,name=message" json:"message,omitempty"`
}

func (m *EnqueueRequest) Reset()                    { *m = EnqueueRequest{} }
func (m *EnqueueRequest) String() string            { return proto.CompactTextString(m) }
func (*EnqueueRequest) ProtoMessage()               {}
func (*EnqueueRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *EnqueueRequest) GetTopic() string {
	if m != nil {
		return m.Topic
	}
	return ""
}

func (m *EnqueueRequest) GetMessage() *TaskMessage {
	if m != nil {
		return m.Message
	}
	return nil
}

type DequeueRequest struct {
	Topic      string `protobuf:"bytes,1,opt,name=topic" json:"topic,omitempty"`
	ClientHost string `protobuf:"bytes,2,opt,name=client_host,json=clientHost" json:"client_host,omitempty"`
}

func (m *DequeueRequest) Reset()                    { *m = DequeueRequest{} }
func (m *DequeueRequest) String() string            { return proto.CompactTextString(m) }
func (*DequeueRequest) ProtoMessage()               {}
func (*DequeueRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *DequeueRequest) GetTopic() string {
	if m != nil {
		return m.Topic
	}
	return ""
}

func (m *DequeueRequest) GetClientHost() string {
	if m != nil {
		return m.ClientHost
	}
	return ""
}

type TaskReply struct {
	Status  string   `protobuf:"bytes,1,opt,name=status" json:"status,omitempty"`
	Message string   `protobuf:"bytes,2,opt,name=message" json:"message,omitempty"`
	Topics  []string `protobuf:"bytes,3,rep,name=topics" json:"topics,omitempty"`
}

func (m *TaskReply) Reset()                    { *m = TaskReply{} }
func (m *TaskReply) String() string            { return proto.CompactTextString(m) }
func (*TaskReply) ProtoMessage()               {}
func (*TaskReply) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *TaskReply) GetStatus() string {
	if m != nil {
		return m.Status
	}
	return ""
}

func (m *TaskReply) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

func (m *TaskReply) GetTopics() []string {
	if m != nil {
		return m.Topics
	}
	return nil
}

type TopicRequest struct {
}

func (m *TopicRequest) Reset()                    { *m = TopicRequest{} }
func (m *TopicRequest) String() string            { return proto.CompactTextString(m) }
func (*TopicRequest) ProtoMessage()               {}
func (*TopicRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func init() {
	proto.RegisterType((*TaskMessage)(nil), "pb.TaskMessage")
	proto.RegisterType((*EnqueueRequest)(nil), "pb.EnqueueRequest")
	proto.RegisterType((*DequeueRequest)(nil), "pb.DequeueRequest")
	proto.RegisterType((*TaskReply)(nil), "pb.TaskReply")
	proto.RegisterType((*TopicRequest)(nil), "pb.TopicRequest")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for TaskQueue service

type TaskQueueClient interface {
	Enqueue(ctx context.Context, in *EnqueueRequest, opts ...grpc.CallOption) (*TaskReply, error)
	Topics(ctx context.Context, in *TopicRequest, opts ...grpc.CallOption) (*TaskReply, error)
	Dequeue(ctx context.Context, in *DequeueRequest, opts ...grpc.CallOption) (*TaskMessage, error)
}

type taskQueueClient struct {
	cc *grpc.ClientConn
}

func NewTaskQueueClient(cc *grpc.ClientConn) TaskQueueClient {
	return &taskQueueClient{cc}
}

func (c *taskQueueClient) Enqueue(ctx context.Context, in *EnqueueRequest, opts ...grpc.CallOption) (*TaskReply, error) {
	out := new(TaskReply)
	err := grpc.Invoke(ctx, "/pb.TaskQueue/Enqueue", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *taskQueueClient) Topics(ctx context.Context, in *TopicRequest, opts ...grpc.CallOption) (*TaskReply, error) {
	out := new(TaskReply)
	err := grpc.Invoke(ctx, "/pb.TaskQueue/Topics", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *taskQueueClient) Dequeue(ctx context.Context, in *DequeueRequest, opts ...grpc.CallOption) (*TaskMessage, error) {
	out := new(TaskMessage)
	err := grpc.Invoke(ctx, "/pb.TaskQueue/Dequeue", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for TaskQueue service

type TaskQueueServer interface {
	Enqueue(context.Context, *EnqueueRequest) (*TaskReply, error)
	Topics(context.Context, *TopicRequest) (*TaskReply, error)
	Dequeue(context.Context, *DequeueRequest) (*TaskMessage, error)

}

func RegisterTaskQueueServer(s *grpc.Server, srv TaskQueueServer) {
	s.RegisterService(&_TaskQueue_serviceDesc, srv)
}

func _TaskQueue_Enqueue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(EnqueueRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TaskQueueServer).Enqueue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.TaskQueue/Enqueue",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TaskQueueServer).Enqueue(ctx, req.(*EnqueueRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TaskQueue_Topics_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TopicRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TaskQueueServer).Topics(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.TaskQueue/Topics",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TaskQueueServer).Topics(ctx, req.(*TopicRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TaskQueue_Dequeue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DequeueRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TaskQueueServer).Dequeue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.TaskQueue/Dequeue",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TaskQueueServer).Dequeue(ctx, req.(*DequeueRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _TaskQueue_serviceDesc = grpc.ServiceDesc{
	ServiceName: "pb.TaskQueue",
	HandlerType: (*TaskQueueServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Enqueue",
			Handler:    _TaskQueue_Enqueue_Handler,
		},
		{
			MethodName: "Topics",
			Handler:    _TaskQueue_Topics_Handler,
		},
		{
			MethodName: "Dequeue",
			Handler:    _TaskQueue_Dequeue_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "message.proto",
}

func init() { proto.RegisterFile("message.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 323 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x92, 0xcd, 0x4a, 0xfb, 0x40,
	0x14, 0xc5, 0x9b, 0xb6, 0xff, 0xf6, 0x9f, 0xdb, 0x0f, 0xe5, 0x22, 0x12, 0xea, 0xc2, 0x32, 0xab,
	0x8a, 0x10, 0xa4, 0xbe, 0x81, 0x28, 0xba, 0x71, 0xd1, 0x10, 0xd7, 0x25, 0x69, 0x2f, 0x1a, 0xfa,
	0x31, 0xd3, 0xce, 0xcd, 0x22, 0xe0, 0x93, 0xf8, 0xb4, 0x32, 0x1f, 0xd5, 0x46, 0x37, 0xee, 0x66,
	0xee, 0xc9, 0x3d, 0xe7, 0x37, 0x87, 0xc0, 0x60, 0x43, 0x5a, 0x67, 0xaf, 0x14, 0xab, 0xbd, 0x64,
	0x89, 0x4d, 0x95, 0x8b, 0x77, 0xe8, 0xa5, 0x99, 0x5e, 0x3d, 0x3b, 0x01, 0x2f, 0x20, 0xe4, 0x4c,
	0xaf, 0xe6, 0x5c, 0x29, 0x8a, 0x82, 0x71, 0x30, 0x09, 0x93, 0xff, 0x66, 0x90, 0x56, 0xea, 0x5b,
	0xcc, 0xe5, 0xb2, 0x8a, 0x9a, 0xe3, 0x60, 0xd2, 0x77, 0xe2, 0x9d, 0x5c, 0x56, 0x78, 0x09, 0xbd,
	0xc5, 0x9e, 0x32, 0xa6, 0x39, 0x17, 0x1b, 0x8a, 0x5a, 0x76, 0x17, 0xdc, 0x28, 0x2d, 0x36, 0x84,
	0x08, 0xed, 0x52, 0xd3, 0x3e, 0x6a, 0x5b, 0xc5, 0x9e, 0xc5, 0x0c, 0x86, 0x0f, 0xdb, 0x5d, 0x49,
	0x25, 0x25, 0xb4, 0x2b, 0x49, 0x33, 0x9e, 0xc1, 0x3f, 0x96, 0xaa, 0x58, 0xf8, 0x70, 0x77, 0xc1,
	0x2b, 0xe8, 0x7a, 0x74, 0x9b, 0xdb, 0x9b, 0x9e, 0xc4, 0x2a, 0x8f, 0x8f, 0xc0, 0x93, 0x83, 0x2e,
	0x1e, 0x61, 0x78, 0x4f, 0x7f, 0xb0, 0x34, 0xbc, 0xeb, 0x82, 0xb6, 0x3c, 0x7f, 0x93, 0x9a, 0xad,
	0xad, 0xe1, 0xb5, 0xa3, 0x27, 0xa9, 0x59, 0xbc, 0x40, 0x68, 0x02, 0x12, 0x52, 0xeb, 0x0a, 0xcf,
	0xa1, 0xa3, 0x39, 0xe3, 0x52, 0x7b, 0x13, 0x7f, 0xc3, 0xa8, 0x0e, 0x16, 0x7e, 0x71, 0x98, 0x0d,
	0x1b, 0xa4, 0xa3, 0xd6, 0xb8, 0x65, 0x36, 0xdc, 0x4d, 0x0c, 0xa1, 0x9f, 0x9a, 0x93, 0xa7, 0x9b,
	0x7e, 0x04, 0x2e, 0x67, 0x66, 0x90, 0x31, 0x86, 0xae, 0x2f, 0x04, 0xd1, 0x3c, 0xb1, 0xde, 0xce,
	0x68, 0x70, 0x78, 0xb6, 0xa5, 0x12, 0x0d, 0xbc, 0x86, 0x8e, 0x75, 0xd3, 0x78, 0x6a, 0xa5, 0x23,
	0xe7, 0xdf, 0x1f, 0xdf, 0x40, 0xd7, 0x57, 0xe3, 0xcc, 0xeb, 0x3d, 0x8d, 0x7e, 0x76, 0x2a, 0x1a,
	0x79, 0xc7, 0xfe, 0x28, 0xb7, 0x9f, 0x01, 0x00, 0x00, 0xff, 0xff, 0x86, 0xed, 0xfb, 0x86, 0x39,
	0x02, 0x00, 0x00,
}
