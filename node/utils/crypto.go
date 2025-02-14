package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	// "sync"
)

// AESKeySize 定义了允许的 AES 密钥长度 (定义允许的AES密钥长度)
type AESKeySize int

const (
	AES128 AESKeySize = 16 // AES128 密钥长度 (AES128密钥长度)
	AES192 AESKeySize = 24 // AES192 密钥长度 (AES192密钥长度)
	AES256 AESKeySize = 32 // AES256 密钥长度 (AES256密钥长度)
)

var (
	// ErrInvalidKeySize 当密钥长度无效时返回 (密钥长度无效时返回的错误)
	ErrInvalidKeySize = errors.New("invalid key size: must be 16, 24, or 32 bytes")
	// ErrCiphertextTooShort 当密文太短时返回 (密文太短时返回的错误)
	ErrCiphertextTooShort = errors.New("ciphertext too short")

	// aesKey 是用于加密和解密的 AES 密钥。 应该只初始化一次。
	// 使用函数返回它确保在使用前已初始化。
	// 考虑使用环境变量或配置文件来加载密钥。
	// (AES密钥，用于加密和解密。应该只初始化一次。强烈建议使用环境变量或配置文件加载密钥，而不是硬编码。)
	aesKey = func() []byte {
		key := []byte("your-invalid-key-with-34-bytes") // 替换为安全的密钥管理策略！ 绝对不要硬编码密钥！
		adjustedKey := adjustKey(key)
		return adjustedKey
	}()

	// aesKeyOnce 确保 aesKey 只初始化一次 (确保aesKey只初始化一次)
	// aesKeyOnce sync.Once
)

// adjustKey 将密钥调整为有效的 AES 密钥长度（16、24 或 32 字节）。
// 如果密钥长度超过 32 字节，则截断密钥；如果密钥长度小于 16 字节，则用零填充密钥。
// (调整密钥长度，使其符合AES的要求。如果过长则截断，过短则填充0。但请注意，填充短密钥是不安全的，应该使用密钥派生函数。)
func adjustKey(key []byte) []byte {
	keyLen := len(key)
	if keyLen >= int(AES256) {
		return key[:AES256] // 截断为 32 字节 (截断为32字节)
	} else if keyLen >= int(AES192) {
		return key[:AES192] // 截断为 24 字节 (截断为24字节)
	} else if keyLen >= int(AES128) {
		return key[:AES128] // 截断为 16 字节 (截断为16字节)
	}

	// 如果密钥太短，则用零填充。 通常不鼓励这样做，而应使用适当的密钥派生函数 (KDF) 来扩展密钥。
	// (如果密钥太短，则填充0。强烈不建议这样做，应该使用密钥派生函数(KDF)来扩展密钥。)
	paddedKey := make([]byte, AES128)
	copy(paddedKey, key)
	return paddedKey
}

// Encrypt 使用 AES 加密明文 (使用AES加密明文)
func Encrypt(plaintext []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", nil // 明文为空时返回空字符串 (明文为空时返回空字符串)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err) // 创建密码块失败 (创建密码块失败)
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize] // 初始化向量 (初始化向量)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err) // 生成 IV 失败 (生成IV失败)
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil // 返回 Base64 编码的密文 (返回Base64编码的密文)
}

// Decrypt 使用 AES 解密密文 (使用AES解密密文)
func Decrypt(ciphertext string) ([]byte, error) {
	if ciphertext == "" {
		return []byte{}, nil // 密文为空时返回空字节切片 (密文为空时返回空字节切片)
	}

	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 ciphertext: %w", err) // 解码 Base64 密文失败 (解码Base64密文失败)
	}

	if len(ciphertextBytes) < aes.BlockSize {
		return nil, ErrCiphertextTooShort // 密文太短 (密文太短)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err) // 创建密码块失败 (创建密码块失败)
	}

	iv := ciphertextBytes[:aes.BlockSize] // 初始化向量 (初始化向量)
	ciphertextBytes = ciphertextBytes[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertextBytes, ciphertextBytes)

	return ciphertextBytes, nil // 返回解密后的明文 (返回解密后的明文)
}
