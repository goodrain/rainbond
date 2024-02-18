package mq

import (
	"sync"
	"time"
)

// KeyValueStore 是一个简单的键值存储结构，支持多值
type KeyValueStore struct {
	data map[string][]string
	mu   sync.Mutex
	cond *sync.Cond
}

// NewKeyValueStore 创建一个新的键值存储实例
func NewKeyValueStore() *KeyValueStore {
	kv := &KeyValueStore{
		data: make(map[string][]string),
	}
	kv.cond = sync.NewCond(&kv.mu)
	return kv
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
	kv.cond.Broadcast()
}

// Get 根据键获取第一个值并删除该键值对中的第一个值
func (kv *KeyValueStore) Get(key string) (string, bool) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	// 如果值列表为空，等待通知或超时
	timeout := time.Now().Add(5 * time.Second)
	for len(kv.data[key]) == 0 {
		select {
		case <-time.After(timeout.Sub(time.Now())):
			return "", false // 超时后返回空值
		default:
			waitChan := make(chan struct{})
			kv.cond.L.Unlock()
			go func() {
				kv.cond.L.Lock()
				close(waitChan)
			}()
			<-waitChan
		}
	}

	// 获取值列表
	values, ok := kv.data[key]
	if ok && len(values) > 0 {
		// 获取第一个值并从切片中删除
		firstValue := values[0]
		if len(values) == 1 {
			delete(kv.data, key) // 如果只有一个值，删除整个键值对
		} else {
			kv.data[key] = values[1:] // 如果有多个值，删除第一个值
		}
		return firstValue, true
	}

	return "", false
}

// Size 返回特定键的值列表大小
func (kv *KeyValueStore) Size(topic string) int64 {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	return int64(len(kv.data[topic]))
}
