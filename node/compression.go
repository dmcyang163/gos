package main

import (
	"errors"
	"sync"

	"github.com/golang/snappy"
)

// 定义压缩池和解压池
var (
	compressorPool   = NewBufferPool() // 用于压缩的缓冲池
	decompressorPool = NewBufferPool() // 用于解压缩的缓冲池
)

// compress 使用 Snappy 压缩数据
func compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty input data")
	}

	// 从 compressorPool 中获取缓冲区
	bufferPtr := compressorPool.Get().(*[]byte) // 获取切片的指针
	defer compressorPool.Put(bufferPtr)         // 使用完毕后放回池中
	buffer := *bufferPtr                        // 解引用指针

	// 如果缓冲区太小，动态调整大小
	if cap(buffer) < len(data) {
		*bufferPtr = make([]byte, len(data)*2) // 调整为输入数据的两倍
		buffer = *bufferPtr
	}

	// 使用 Snappy 压缩数据
	compressed := snappy.Encode(buffer, data)
	return compressed, nil
}

// decompress 使用 Snappy 解压缩数据
func decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty input data")
	}

	// 从 decompressorPool 中获取缓冲区
	bufferPtr := decompressorPool.Get().(*[]byte) // 获取切片的指针
	defer decompressorPool.Put(bufferPtr)         // 使用完毕后放回池中
	buffer := *bufferPtr                          // 解引用指针

	// 如果缓冲区太小，动态调整大小
	if cap(buffer) < len(data)*2 { // 解压缩后数据可能更大
		*bufferPtr = make([]byte, len(data)*4) // 调整为输入数据的四倍
		buffer = *bufferPtr
	}

	// 使用 Snappy 解压缩数据
	decoded, err := snappy.Decode(buffer, data)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

// NewBufferPool 创建一个新的 sync.Pool，用于复用 []byte 的指针
func NewBufferPool() *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 1024*1024*4) // 初始缓冲区大小为 4M 字节
			return &buf                      // 返回切片的指针
		},
	}
}
