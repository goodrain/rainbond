package mq

import (
	"context"
	"sync"
	"time"
)

// KeyValueStore 是一个简单的键值存储结构，支持多值
type KeyValueStore struct {
	data map[string][]string
	mu   sync.Mutex
}

// NewKeyValueStore 创建一个新的键值存储实例
func NewKeyValueStore() *KeyValueStore {
	return &KeyValueStore{
		data: make(map[string][]string),
	}
}

// Put 将键值对放入存储
func (kv *KeyValueStore) Put(key, value string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if existingValues, ok := kv.data[key]; ok {
		kv.data[key] = append(existingValues, value)
	} else {
		kv.data[key] = []string{value}
	}
}

// Get 根据键获取第一个值并删除该键值对中的第一个值
func (kv *KeyValueStore) Get(ctx context.Context, key string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		kv.mu.Lock()
		// Recheck cancellation while holding the lock before removing a message.
		if err := ctx.Err(); err != nil {
			kv.mu.Unlock()
			return "", err
		}
		values, ok := kv.data[key]
		if ok && len(values) > 0 {
			// 获取第一个值并从切片中删除
			firstValue := values[0]
			if len(values) == 1 {
				delete(kv.data, key)
			} else {
				kv.data[key] = values[1:]
			}
			kv.mu.Unlock()
			return firstValue, nil
		}
		kv.mu.Unlock()

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

// Size 返回特定键的值列表大小
func (kv *KeyValueStore) Size(topic string) int64 {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	return int64(len(kv.data[topic]))
}
