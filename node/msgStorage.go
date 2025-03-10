package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

// 使用 jsoniter 替代标准库的 encoding/json

// MsgStorage 定义消息存储的接口
type MsgStorage interface {
	Create(msg Message) error                    // 插入一条新消息
	Read(msgType, msgID string) (Message, error) // 读取指定消息
	Update(msg Message) error                    // 更新指定消息
	Delete(msgType, msgID string) error          // 删除指定消息
	List(msgType string) ([]Message, error)      // 列出指定类型的消息
	Close() error                                // 关闭存储
}

// BoltMsgStorage 是基于 bbolt 的消息存储实现
type BoltMsgStorage struct {
	db *bbolt.DB
	mu sync.Mutex // 用于保护共享状态（如果有）
}

// NewBoltMsgStorage 创建一个新的 BoltMsgStorage 实例
func NewBoltMsgStorage(dbPath string) (*BoltMsgStorage, error) {
	// 确保文件夹存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// 以读写模式打开数据库
	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bbolt database: %w", err)
	}

	return &BoltMsgStorage{db: db}, nil
}

// Create 插入一条新消息
func (s *BoltMsgStorage) Create(msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		// 根据消息类型创建或获取 bucket
		bucket, err := tx.CreateBucketIfNotExists([]byte(msg.Type))
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}

		// 生成唯一的消息 ID（如果未提供）
		if msg.ID == "" {
			msg.ID = fmt.Sprintf("%d", time.Now().UnixNano())
		}

		// 使用 jsoniter 序列化 Message
		messageBytes, err := jniter.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}

		// 保存消息到 bucket
		return bucket.Put([]byte(msg.ID), messageBytes)
	})
}

// Read 读取指定消息
func (s *BoltMsgStorage) Read(msgType, msgID string) (Message, error) {
	var msg Message

	err := s.db.View(func(tx *bbolt.Tx) error {
		// 获取指定类型的 bucket
		bucket := tx.Bucket([]byte(msgType))
		if bucket == nil {
			return fmt.Errorf("bucket not found: %s", msgType)
		}

		// 读取消息
		messageBytes := bucket.Get([]byte(msgID))
		if messageBytes == nil {
			return fmt.Errorf("message not found: %s", msgID)
		}

		// 使用 jsoniter 反序列化 Message
		return jniter.Unmarshal(messageBytes, &msg)
	})

	return msg, err
}

// Update 更新指定消息
func (s *BoltMsgStorage) Update(msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		// 获取指定类型的 bucket
		bucket := tx.Bucket([]byte(msg.Type))
		if bucket == nil {
			return fmt.Errorf("bucket not found: %s", msg.Type)
		}

		// 检查消息是否存在
		if bucket.Get([]byte(msg.ID)) == nil {
			return fmt.Errorf("message not found: %s", msg.ID)
		}

		// 使用 jsoniter 序列化 Message
		messageBytes, err := jniter.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}

		// 更新消息
		return bucket.Put([]byte(msg.ID), messageBytes)
	})
}

// Delete 删除指定消息
func (s *BoltMsgStorage) Delete(msgType, msgID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		// 获取指定类型的 bucket
		bucket := tx.Bucket([]byte(msgType))
		if bucket == nil {
			return fmt.Errorf("bucket not found: %s", msgType)
		}

		// 检查消息是否存在
		if bucket.Get([]byte(msgID)) == nil {
			return fmt.Errorf("message not found: %s", msgID)
		}

		// 删除消息
		return bucket.Delete([]byte(msgID))
	})
}

// List 列出指定类型的消息
func (s *BoltMsgStorage) List(msgType string) ([]Message, error) {
	var messages []Message

	err := s.db.View(func(tx *bbolt.Tx) error {
		// 获取指定类型的 bucket
		bucket := tx.Bucket([]byte(msgType))
		if bucket == nil {
			return nil // 如果 bucket 不存在，返回空列表
		}

		// 遍历 bucket 中的所有消息
		return bucket.ForEach(func(k, v []byte) error {
			var msg Message
			if err := jniter.Unmarshal(v, &msg); err != nil {
				return fmt.Errorf("failed to unmarshal message: %w", err)
			}
			messages = append(messages, msg)
			return nil
		})
	})

	return messages, err
}

// Close 关闭 bbolt 数据库
func (s *BoltMsgStorage) Close() error {
	return s.db.Close()
}
