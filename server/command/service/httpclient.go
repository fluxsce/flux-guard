package service

import (
	"net"
	"net/http"
	"sync"
	"time"

	"fluxguard/server/utils/const/limits"
)

// 短拨共用 Transport，避免每次 New Transport 泄漏空闲连接。
var (
	appsideDialOnce sync.Once
	appsideDialHTTP *http.Client
)

func appsideHTTPClient(timeout time.Duration) *http.Client {
	appsideDialOnce.Do(func() {
		appsideDialHTTP = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   limits.DialConnect,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:          32,
				MaxIdleConnsPerHost:   8,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				ForceAttemptHTTP2:     true,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	})
	// 每请求自带超时：复制 Client，共用 Transport。
	return &http.Client{
		Transport:     appsideDialHTTP.Transport,
		Timeout:       timeout,
		CheckRedirect: appsideDialHTTP.CheckRedirect,
	}
}
