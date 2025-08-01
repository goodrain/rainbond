package util

import (
	"github.com/sirupsen/logrus"
	"runtime"
	"sync"
	"time"
)

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	mu sync.RWMutex

	// 计数器
	TotalMessages     int64
	ProcessedMessages int64
	DroppedMessages   int64
	PoolHits          int64
	PoolMisses        int64

	// 内存统计
	AllocatedMemory uint64
	GCCount         uint32

	// 延迟统计
	AvgProcessTime time.Duration
	MaxProcessTime time.Duration

	lastUpdate       time.Time
	processTimeSum   time.Duration
	processTimeCount int64
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		lastUpdate: time.Now(),
	}
}

// RecordMessage 记录处理的消息
func (pm *PerformanceMonitor) RecordMessage(processTime time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.TotalMessages++
	pm.ProcessedMessages++

	// 更新处理时间统计
	pm.processTimeSum += processTime
	pm.processTimeCount++
	pm.AvgProcessTime = pm.processTimeSum / time.Duration(pm.processTimeCount)

	if processTime > pm.MaxProcessTime {
		pm.MaxProcessTime = processTime
	}
}

// RecordDroppedMessage 记录丢弃的消息
func (pm *PerformanceMonitor) RecordDroppedMessage() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.TotalMessages++
	pm.DroppedMessages++
}

// RecordPoolHit 记录对象池命中
func (pm *PerformanceMonitor) RecordPoolHit() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.PoolHits++
}

// RecordPoolMiss 记录对象池未命中
func (pm *PerformanceMonitor) RecordPoolMiss() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.PoolMisses++
}

// UpdateMemoryStats 更新内存统计
func (pm *PerformanceMonitor) UpdateMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.AllocatedMemory = m.Alloc
	pm.GCCount = m.NumGC
}

// GetStats 获取统计信息
func (pm *PerformanceMonitor) GetStats() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pm.UpdateMemoryStats()

	stats := map[string]interface{}{
		"total_messages":     pm.TotalMessages,
		"processed_messages": pm.ProcessedMessages,
		"dropped_messages":   pm.DroppedMessages,
		"drop_rate":          float64(pm.DroppedMessages) / float64(pm.TotalMessages),
		"pool_hits":          pm.PoolHits,
		"pool_misses":        pm.PoolMisses,
		"pool_hit_rate":      float64(pm.PoolHits) / float64(pm.PoolHits+pm.PoolMisses),
		"allocated_memory":   pm.AllocatedMemory,
		"gc_count":           pm.GCCount,
		"avg_process_time":   pm.AvgProcessTime.Nanoseconds(),
		"max_process_time":   pm.MaxProcessTime.Nanoseconds(),
	}

	return stats
}

// LogStats 定期记录统计信息
func (pm *PerformanceMonitor) LogStats(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats := pm.GetStats()
			logrus.WithFields(logrus.Fields{
				"total_messages":      stats["total_messages"],
				"processed_messages":  stats["processed_messages"],
				"dropped_messages":    stats["dropped_messages"],
				"drop_rate":           stats["drop_rate"],
				"pool_hit_rate":       stats["pool_hit_rate"],
				"allocated_memory_mb": stats["allocated_memory"].(uint64) / 1024 / 1024,
				"gc_count":            stats["gc_count"],
				"avg_process_time_us": stats["avg_process_time"].(int64) / 1000,
				"max_process_time_us": stats["max_process_time"].(int64) / 1000,
			}).Info("EventLog Performance Stats")
		}
	}
}

// Reset 重置统计信息
func (pm *PerformanceMonitor) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.TotalMessages = 0
	pm.ProcessedMessages = 0
	pm.DroppedMessages = 0
	pm.PoolHits = 0
	pm.PoolMisses = 0
	pm.AllocatedMemory = 0
	pm.GCCount = 0
	pm.AvgProcessTime = 0
	pm.MaxProcessTime = 0
	pm.processTimeSum = 0
	pm.processTimeCount = 0
	pm.lastUpdate = time.Now()
}
