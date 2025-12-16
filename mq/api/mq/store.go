package mq

import (
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
func (kv *KeyValueStore) Get(key string) (string, bool) {
	deadline := time.Now().Add(5 * time.Second)

	for {
		// 检查是否超时
		if time.Now().After(deadline) {
			return "", false
		}

		kv.mu.Lock()
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
			return firstValue, true
		}
		kv.mu.Unlock()

		// 短暂休眠后重试，避免忙等待
		time.Sleep(100 * time.Millisecond)
	}
}

// Size 返回特定键的值列表大小
func (kv *KeyValueStore) Size(topic string) int64 {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	return int64(len(kv.data[topic]))
}
