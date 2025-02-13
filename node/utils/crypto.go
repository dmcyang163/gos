package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

func adjustKey(key []byte) []byte {
	if len(key) >= 32 {
		return key[:32] // 截取前 32 字节
	} else if len(key) >= 24 {
		return key[:24] // 截取前 24 字节
	} else if len(key) >= 16 {
		return key[:16] // 截取前 16 字节
	}
	// 如果密钥长度不足，填充到 16 字节
	paddedKey := make([]byte, 16)
	copy(paddedKey, key)
	return paddedKey
}

var aesKey = adjustKey([]byte("your-invalid-key-with-34-bytes"))

// validateKey 验证密钥长度是否符合 AES 要求
func validateKey(key []byte) error {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return fmt.Errorf("invalid key size: %d (must be 16, 24, or 32 bytes)", len(key))
	}
	return nil
}

// Encrypt 使用 AES 加密消息
func Encrypt(plaintext []byte) (string, error) {
	// 如果明文为空，直接返回空字符串
	if len(plaintext) == 0 {
		return "", nil
	}

	// 验证密钥长度
	if err := validateKey(aesKey); err != nil {
		return "", fmt.Errorf("invalid key: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err)
	}

	// 创建加密块
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize] // 初始化向量
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	// 加密
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	// 返回 Base64 编码的密文
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 使用 AES 解密消息
func Decrypt(ciphertext string) ([]byte, error) {
	// 如果密文为空，直接返回空字节切片
	if ciphertext == "" {
		return []byte{}, nil
	}

	// 验证密钥长度
	if err := validateKey(aesKey); err != nil {
		return nil, fmt.Errorf("invalid key: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	// 解码 Base64 密文
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 ciphertext: %w", err)
	}

	if len(ciphertextBytes) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	// 提取初始化向量
	iv := ciphertextBytes[:aes.BlockSize]
	ciphertextBytes = ciphertextBytes[aes.BlockSize:]

	// 解密
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertextBytes, ciphertextBytes)

	// 返回解密后的明文
	return ciphertextBytes, nil
}
