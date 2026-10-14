// Package crypto 是两边共用的对称加解密。
// 密文带 ENCY_ 前缀，算法是 AES-GCM，密钥由字符串经 SHA-256 派生。
// 只用于必须还原原文的配置和密钥。登录口令用 server/utils/secret 的 bcrypt，不能放进这里。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"strings"
)

const (
	prefix  = "ENCY_"
	version = byte(0x01)
)

// DefaultKey 是开发默认密钥。调用方传入自己的密钥时不用它。
const DefaultKey = "fluxguard-default-encryption-key-please-change"

// IsEncrypted 判断字符串是否带 ENCY_ 前缀。
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, prefix)
}

// Encrypt 加密明文。每次调用的 nonce 不同，密文不会重复。
func Encrypt(secretKey, plaintext string) (string, error) {
	if secretKey == "" {
		secretKey = DefaultKey
	}
	block, err := aes.NewCipher(derive(secretKey))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return prefix + pack(nonce, sealed), nil
}

// Decrypt 解密 ENCY_ 密文。没有前缀时按原文返回，方便兼容历史明文。
func Decrypt(secretKey, stored string) (string, error) {
	if !IsEncrypted(stored) {
		return stored, nil
	}
	if secretKey == "" {
		secretKey = DefaultKey
	}
	nonce, ciphertext, err := unpack(strings.TrimPrefix(stored, prefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(derive(secretKey))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(nonce) != gcm.NonceSize() {
		return "", errors.New("密文格式无效")
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func derive(secretKey string) []byte {
	sum := sha256.Sum256([]byte(secretKey))
	return sum[:]
}

func pack(nonce, ciphertext []byte) string {
	buf := make([]byte, 1+2+len(nonce)+4+len(ciphertext)+2)
	pos := 0
	buf[pos] = version
	pos++
	binary.BigEndian.PutUint16(buf[pos:pos+2], uint16(len(nonce)))
	pos += 2
	copy(buf[pos:pos+len(nonce)], nonce)
	pos += len(nonce)
	binary.BigEndian.PutUint32(buf[pos:pos+4], uint32(len(ciphertext)))
	pos += 4
	copy(buf[pos:pos+len(ciphertext)], ciphertext)
	pos += len(ciphertext)
	binary.BigEndian.PutUint16(buf[pos:pos+2], 0)
	return base64.StdEncoding.EncodeToString(buf)
}

func unpack(encoded string) (nonce, ciphertext []byte, err error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, nil, err
	}
	if len(data) < 1+2+4+2 || data[0] != version {
		return nil, nil, errors.New("密文格式无效")
	}
	pos := 1
	nonceLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	if nonceLen <= 0 || len(data) < pos+nonceLen+4 {
		return nil, nil, errors.New("密文格式无效")
	}
	nonce = data[pos : pos+nonceLen]
	pos += nonceLen
	cipherLen := int(binary.BigEndian.Uint32(data[pos : pos+4]))
	pos += 4
	if cipherLen < 0 || len(data) < pos+cipherLen+2 {
		return nil, nil, errors.New("密文格式无效")
	}
	ciphertext = data[pos : pos+cipherLen]
	return nonce, ciphertext, nil
}
