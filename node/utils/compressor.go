package utils

import (
	"sync"

	"github.com/klauspost/compress/snappy"
	"github.com/klauspost/compress/zstd"
)

// 默认压缩器
var defaultCompressor = NewZstdCompressor()

// Compress 使用默认压缩器压缩数据
func Compress(data []byte) ([]byte, error) {
	return defaultCompressor.Compress(data)
}

// Decompress 使用默认压缩器解压缩数据
func Decompress(data []byte) ([]byte, error) {
	return defaultCompressor.Decompress(data)
}

// 定义压缩池和解压池
var (
	compressorPool   = NewBufferPool() // 用于压缩的缓冲池
	decompressorPool = NewBufferPool() // 用于解压缩的缓冲池
)

// Compressor 是压缩器的接口
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
}

// SnappyCompressor 实现 Snappy 压缩算法
type SnappyCompressor struct{}

func NewSnappyCompressor() *SnappyCompressor {
	return &SnappyCompressor{}
}

func (s *SnappyCompressor) Compress(data []byte) ([]byte, error) {
	buf := compressorPool.Get().(*[]byte)
	defer compressorPool.Put(buf)

	// 动态调整缓冲区大小
	if cap(*buf) < len(data) {
		*buf = make([]byte, 0, len(data)*2)
	} else {
		*buf = (*buf)[:0]
	}

	compressed := snappy.Encode(*buf, data)
	return compressed, nil
}

func (s *SnappyCompressor) Decompress(data []byte) ([]byte, error) {
	buf := decompressorPool.Get().(*[]byte)
	defer decompressorPool.Put(buf)

	// 动态调整缓冲区大小
	if cap(*buf) < len(data) {
		*buf = make([]byte, 0, len(data)*4)
	} else {
		*buf = (*buf)[:0]
	}

	decompressed, err := snappy.Decode(*buf, data)
	if err != nil {
		return nil, err
	}
	return decompressed, nil
}

// ZstdCompressor 实现 Zstd 压缩算法
type ZstdCompressor struct {
	encoder *zstd.Encoder
	decoder *zstd.Decoder
}

func NewZstdCompressor() *ZstdCompressor {
	encoder, _ := zstd.NewWriter(nil)
	decoder, _ := zstd.NewReader(nil)
	return &ZstdCompressor{
		encoder: encoder,
		decoder: decoder,
	}
}

func (z *ZstdCompressor) Compress(data []byte) ([]byte, error) {
	buf := compressorPool.Get().(*[]byte)
	defer compressorPool.Put(buf)

	// 动态调整缓冲区大小
	if cap(*buf) < len(data) {
		*buf = make([]byte, 0, len(data)*2)
	} else {
		*buf = (*buf)[:0]
	}

	compressed := z.encoder.EncodeAll(data, *buf)
	return compressed, nil
}

func (z *ZstdCompressor) Decompress(data []byte) ([]byte, error) {
	buf := decompressorPool.Get().(*[]byte)
	defer decompressorPool.Put(buf)

	// 动态调整缓冲区大小
	if cap(*buf) < len(data) {
		*buf = make([]byte, 0, len(data)*4)
	} else {
		*buf = (*buf)[:0]
	}

	decompressed, err := z.decoder.DecodeAll(data, *buf)
	if err != nil {
		return nil, err
	}
	return decompressed, nil
}

// NewBufferPool 创建一个新的 sync.Pool，用于管理字节切片
func NewBufferPool() *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 0, 1024*1024*4) // 初始容量为 4M
			return &buf
		},
	}
}

// GetBuffer从指定的缓冲池中获取缓冲区，并确保其至少具有指定的大小
func GetBuffer(pool *sync.Pool, size int) ([]byte, func()) {
	bufferPtr := pool.Get().(*[]byte)
	buffer := *bufferPtr

	if len(buffer) < size {
		buffer = make([]byte, size)
	} else {
		buffer = buffer[:size]
	}

	releaseBuffer := func() {
		pool.Put(&buffer)
	}

	return buffer, releaseBuffer
}
