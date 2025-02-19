package utils

import (
	"fmt"
	"sync"

	"github.com/klauspost/compress/snappy"
	"github.com/klauspost/compress/zstd"
)

// 默认压缩器
var (
	compressor   Compressor = NewZstdCompressor()
	compressorMu sync.Mutex
)

// Compress 使用默认压缩器压缩数据
func Compress(data []byte) ([]byte, error) {
	compressorMu.Lock()
	defer compressorMu.Unlock()
	return compressor.Compress(data)
}

// Decompress 使用默认压缩器解压缩数据
func Decompress(data []byte) ([]byte, error) {
	compressorMu.Lock()
	defer compressorMu.Unlock()
	return compressor.Decompress(data)
}

// Compressor 定义压缩器接口
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
}

// SnappyCompressor 使用 Snappy 算法实现压缩
type SnappyCompressor struct{}

// NewSnappyCompressor 创建 Snappy 压缩器
func NewSnappyCompressor() *SnappyCompressor {
	return &SnappyCompressor{}
}

// Compress 压缩数据
func (s *SnappyCompressor) Compress(data []byte) ([]byte, error) {
	return snappy.Encode(nil, data), nil
}

// Decompress 解压缩数据
func (s *SnappyCompressor) Decompress(data []byte) ([]byte, error) {
	decoded, err := snappy.Decode(nil, data)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

// ZstdCompressor 使用 Zstd 算法实现压缩
type ZstdCompressor struct {
	encoder *zstd.Encoder
	decoder *zstd.Decoder
	mu      sync.Mutex
}

// NewZstdCompressor 创建 Zstd 压缩器
func NewZstdCompressor() *ZstdCompressor {
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize zstd encoder: %v", err))
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize zstd decoder: %v", err))
	}

	return &ZstdCompressor{
		encoder: encoder,
		decoder: decoder,
	}
}

// Compress 压缩数据
func (z *ZstdCompressor) Compress(data []byte) ([]byte, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	return z.encoder.EncodeAll(data, nil), nil
}

// Decompress 解压缩数据
func (z *ZstdCompressor) Decompress(data []byte) ([]byte, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	decoded, err := z.decoder.DecodeAll(data, nil)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

// NewBufferPool 创建缓冲区池
func NewBufferPool(size int) *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 0, size)
			return &buf
		},
	}
}

// GetBuffer 从缓冲区池中获取缓冲区
func GetBuffer(pool *sync.Pool, size int) ([]byte, func()) {
	const maxBufferSize = 1 << 30 // 1GB

	if size <= 0 || size > maxBufferSize {
		return nil, func() {}
	}

	// 从池中获取缓冲区
	bufferPtr := pool.Get().(*[]byte)
	buffer := *bufferPtr

	if cap(buffer) >= size {
		buffer = buffer[:size]
		return buffer, func() {
			*bufferPtr = buffer[:0] // 重置切片
			pool.Put(bufferPtr)
		}
	}

	// 创建新缓冲区
	newBuffer := make([]byte, size)
	return newBuffer, func() {
		pool.Put(&newBuffer)
	}
}
