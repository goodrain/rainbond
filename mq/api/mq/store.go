package mq

import (
	"sync"
	"time"
)

// KeyValueStore 是一个简单的键值存储结构，支持多值
type KeyValueStore struct {
	data  map[string][]string
	mu    sync.RWMutex          // 使用读写锁
	conds map[string]*sync.Cond // 每个键有一个条件变量
}

// NewKeyValueStore 创建一个新的键值存储实例
func NewKeyValueStore() *KeyValueStore {
	kv := &KeyValueStore{
		data:  make(map[string][]string),
		conds: make(map[string]*sync.Cond),
	}
	return kv
}

// getCond 获取或创建指定键的条件变量
func (kv *KeyValueStore) getCond(key string) *sync.Cond {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if _, ok := kv.conds[key]; !ok {
		kv.conds[key] = sync.NewCond(&sync.Mutex{})
	}
	return kv.conds[key]
}

// Put 将键值对放入存储
func (kv *KeyValueStore) Put(key, value string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	// 如果键已存在，则追加值；否则，创建新的值切片
	if existingValues, ok := kv.data[key]; ok {
		kv.data[key] = append(existingValues, value)
	} else {
		kv.data[key] = []string{value}
	}

	// 通知等待在该键上的所有 Goroutine
	if cond, exists := kv.conds[key]; exists {
		cond.L.Lock()
		cond.Broadcast()
		cond.L.Unlock()
	}
}

// Get 根据键获取第一个值并删除该键值对中的第一个值
func (kv *KeyValueStore) Get(key string) (string, bool) {
	cond := kv.getCond(key)
	cond.L.Lock() // 锁定条件变量的互斥锁
	defer cond.L.Unlock()

	// 设置超时，仅创建一次定时器
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	for {
		kv.mu.RLock() // 使用读锁来访问数据
		values, ok := kv.data[key]
		if ok && len(values) > 0 {
			// 获取第一个值并从切片中删除
			firstValue := values[0]
			kv.mu.RUnlock()
			// 获取写锁来进行删除操作
			kv.mu.Lock()
			if len(values) == 1 {
				delete(kv.data, key) // 如果只有一个值，删除整个键值对
			} else {
				kv.data[key] = values[1:] // 如果有多个值，删除第一个值
			}
			kv.mu.Unlock() // 释放写锁
			return firstValue, true
		}
		kv.mu.RUnlock()

		// 如果没有数据，等待条件变量通知或超时
		select {
		case <-timer.C:
			return "", false // 超时后返回空值
		default:
			cond.Wait() // 等待条件变量通知
		}
	}
}

// Size 返回特定键的值列表大小
func (kv *KeyValueStore) Size(topic string) int64 {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	return int64(len(kv.data[topic]))
}
