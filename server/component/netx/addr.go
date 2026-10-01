// Package netx 把页面上填写的地址收成可拨号的 host:port 或 http 基址。
package netx

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// HostPort 收成单个 host:port。没有端口时用 defaultPort。
func HostPort(address string, defaultPort int) (string, error) {
	list, err := HostPorts(address, defaultPort)
	if err != nil {
		return "", err
	}
	if strings.Contains(address, ",") {
		return "", fmt.Errorf("只填写一个地址")
	}
	return list[0], nil
}

// HostPorts 按逗号拆开多个 host:port。Kafka broker 列表用这个。
func HostPorts(address string, defaultPort int) ([]string, error) {
	raw := strings.TrimSpace(address)
	if raw == "" {
		return nil, fmt.Errorf("请填写地址")
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		host := hostOf(part)
		if host == "" {
			return nil, fmt.Errorf("地址无效")
		}
		if _, _, err := net.SplitHostPort(host); err != nil {
			if defaultPort <= 0 {
				return nil, fmt.Errorf("地址需要端口")
			}
			host = net.JoinHostPort(host, strconv.Itoa(defaultPort))
		}
		out = append(out, host)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("请填写地址")
	}
	return out, nil
}

// BaseURL 收成不带路径的 http(s) 基址，供 Nacos 拼接健康检查。
func BaseURL(address string, defaultPort int) (string, error) {
	raw := strings.TrimSpace(address)
	if raw == "" {
		return "", fmt.Errorf("请填写地址")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("地址无效")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("只支持 http 或 https")
	}
	host := u.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		if defaultPort <= 0 {
			return "", fmt.Errorf("地址需要端口")
		}
		host = net.JoinHostPort(host, strconv.Itoa(defaultPort))
	}
	return u.Scheme + "://" + host, nil
}

func hostOf(address string) string {
	if strings.Contains(address, "://") {
		u, err := url.Parse(address)
		if err != nil {
			return ""
		}
		return u.Host
	}
	return address
}
