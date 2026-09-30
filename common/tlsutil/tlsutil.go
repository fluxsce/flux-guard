// Package tlsutil 加载服务端 TLS 证书（PEM）。字段对齐 gateway：证书文件、私钥、可选密码。
// 当前支持未加密 PEM；若私钥为 PKCS#8 加密格式，须先用 openssl 解密或提供兼容的明文私钥。
package tlsutil

import (
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

// Files 是磁盘上的证书与私钥。
type Files struct {
	CertFile    string
	KeyFile     string
	KeyPassword string
}

// ServerConfig 生成仅含本机证书、最低 TLS1.2 的服务端配置。
func ServerConfig(f Files) (*tls.Config, error) {
	cert, err := loadPair(f)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func loadPair(f Files) (tls.Certificate, error) {
	certPEM, err := os.ReadFile(f.CertFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("读取证书 %s: %w", f.CertFile, err)
	}
	keyPEM, err := os.ReadFile(f.KeyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("读取私钥 %s: %w", f.KeyFile, err)
	}
	if err := rejectEncryptedKey(keyPEM, f.KeyPassword); err != nil {
		return tls.Certificate{}, err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("加载证书与私钥: %w", err)
	}
	return cert, nil
}

func rejectEncryptedKey(keyPEM []byte, password string) error {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil
	}
	encrypted := block.Type == "ENCRYPTED PRIVATE KEY" || block.Headers["DEK-Info"] != ""
	if !encrypted {
		return nil
	}
	if strings.TrimSpace(password) != "" {
		return fmt.Errorf("暂不支持进程内解密加密私钥，请先用 openssl 解密后再填 key_file")
	}
	return fmt.Errorf("私钥已加密，须先解密，或配置未加密的 PEM 私钥")
}
