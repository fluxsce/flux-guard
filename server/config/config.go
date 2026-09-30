// Package config 读取中央服务器的 YAML 配置。缺文件时使用开发默认值。
// 产品版本不写在这份配置里，编译时写入 common/buildinfo。
// 数据库连接串由 server/store 根据 db 生成。
package config

import (
	"crypto/tls"
	"fmt"
	"strings"

	"fluxguard/common/logger"
	"fluxguard/common/tlsutil"
	"fluxguard/common/yamlcfg"
)

// Server 是中央服务器配置。
type Server struct {
	Listen   string        `mapstructure:"listen"`
	TenantID string        `mapstructure:"tenant_id"`
	DB       DB            `mapstructure:"db"`
	Log      logger.Config `mapstructure:"log"`

	// EnableHTTPS 为 true 时以 TLS 监听，须配置 CertFile 与 KeyFile。
	EnableHTTPS bool `mapstructure:"enable_https"`
	// CertFile / KeyFile 是 PEM 证书与私钥路径。
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	// KeyPassword 解密加密私钥；未加密留空。
	KeyPassword string `mapstructure:"key_password"`
	// CookieSecure 控制会话 Cookie 的 Secure。nil 表示跟随 EnableHTTPS。
	CookieSecure *bool `mapstructure:"cookie_secure"`
}

// DB 是 db 下读到的数据库配置，原样保留，不在这里拼连接串。
type DB struct {
	Driver   string `mapstructure:"driver"`
	DSN      string `mapstructure:"dsn"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

var loaded Server

// TenantID 返回本进程所属租户。尚未加载配置时为 default。
func TenantID() string {
	if id := strings.TrimSpace(loaded.TenantID); id != "" {
		return id
	}
	return "default"
}

// SessionCookieSecure 返回会话 Cookie 是否仅 HTTPS 发送。
// cookie_secure 未写时跟随 enable_https。
func (c Server) SessionCookieSecure() bool {
	if c.CookieSecure != nil {
		return *c.CookieSecure
	}
	return c.EnableHTTPS
}

// TLSConfig 在启用 HTTPS 时加载证书。未启用返回 nil。
func (c Server) TLSConfig() (*tls.Config, error) {
	if !c.EnableHTTPS {
		return nil, nil
	}
	certFile := strings.TrimSpace(c.CertFile)
	keyFile := strings.TrimSpace(c.KeyFile)
	if certFile == "" || keyFile == "" {
		return nil, fmt.Errorf("启用 HTTPS 时须配置 cert_file 与 key_file")
	}
	return tlsutil.ServerConfig(tlsutil.Files{
		CertFile:    certFile,
		KeyFile:     keyFile,
		KeyPassword: c.KeyPassword,
	})
}

// Load 读取配置。path 为空时使用 configs/server.yaml。只读本机 yaml。
func Load(path string) (Server, error) {
	cfg := Server{Listen: ":8080", TenantID: "default"}
	cfg.DB.Driver = "sqlite"
	cfg.Log = logger.Defaults("logs/server.log")
	if path == "" {
		path = "configs/server.yaml"
	}
	if err := yamlcfg.Read(path, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	if strings.TrimSpace(cfg.TenantID) == "" {
		cfg.TenantID = "default"
	}
	if strings.TrimSpace(cfg.DB.Driver) == "" {
		cfg.DB.Driver = "sqlite"
	}
	loaded = cfg
	return cfg, nil
}
