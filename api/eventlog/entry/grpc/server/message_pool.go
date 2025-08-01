package server

import (
	"github.com/goodrain/rainbond/api/eventlog/entry/grpc/pb"
	"sync"
)

// LogMessagePool LogMessage对象池，减少内存分配
type LogMessagePool struct {
	pool sync.Pool
}

// NewLogMessagePool 创建新的LogMessage对象池
func NewLogMessagePool() *LogMessagePool {
	return &LogMessagePool{
		pool: sync.Pool{
			New: func() interface{} {
				return &pb.LogMessage{}
			},
		},
	}
}

// Get 从对象池获取LogMessage
func (p *LogMessagePool) Get() *pb.LogMessage {
	return p.pool.Get().(*pb.LogMessage)
}

// Put 将LogMessage归还到对象池
func (p *LogMessagePool) Put(msg *pb.LogMessage) {
	if msg != nil {
		// 重置对象状态，避免数据污染
		msg.Reset()
		p.pool.Put(msg)
	}
}

// BatchBuffer 批处理缓冲区
type BatchBuffer struct {
	Messages []*pb.LogMessage
	Size     int
	mu       sync.Mutex
}

// NewBatchBuffer 创建批处理缓冲区
func NewBatchBuffer(capacity int) *BatchBuffer {
	return &BatchBuffer{
		Messages: make([]*pb.LogMessage, 0, capacity),
		Size:     0,
	}
}

// Add 添加消息到批处理缓冲区
func (b *BatchBuffer) Add(msg *pb.LogMessage) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.Size >= cap(b.Messages) {
		return false // 缓冲区已满
	}

	b.Messages = append(b.Messages, msg)
	b.Size++
	return true
}

// Flush 清空缓冲区并返回所有消息
func (b *BatchBuffer) Flush() []*pb.LogMessage {
	b.mu.Lock()
	defer b.mu.Unlock()

	result := make([]*pb.LogMessage, b.Size)
	copy(result, b.Messages)

	// 重置缓冲区
	b.Messages = b.Messages[:0]
	b.Size = 0

	return result
}

// IsFull 检查缓冲区是否已满
func (b *BatchBuffer) IsFull() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Size >= cap(b.Messages)
}
