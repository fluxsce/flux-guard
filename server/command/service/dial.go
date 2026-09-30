package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"fluxguard/protocol"
	"fluxguard/server/command/models"
	"fluxguard/server/limits"
)

var errDial = errors.New("拨号未接通")

func dialEndpoint(addr string) (string, net.IP, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", nil, models.ErrNotFound
	}
	if strings.Contains(addr, "://") {
		parsed, err := url.Parse(addr)
		if err != nil {
			return "", nil, err
		}
		if parsed.Scheme != "http" {
			return "", nil, models.ErrNotFound
		}
		addr = parsed.Host
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "", nil, models.ErrNotFound
	}
	return "http://" + net.JoinHostPort(ip.String(), port) + protocol.ExecPath, ip, nil
}

func allowedTarget(addr, primary, list string) bool {
	_, ip, err := dialEndpoint(addr)
	if err != nil || ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	text := ip.String()
	if text == strings.TrimSpace(primary) {
		return true
	}
	for _, part := range strings.Split(list, ",") {
		if strings.TrimSpace(part) == text {
			return true
		}
	}
	return false
}

func postExec(ctx context.Context, endpoint, key string, call protocol.CommandCall) (protocol.ExecReply, error) {
	raw, err := json.Marshal(call)
	if err != nil {
		return protocol.ExecReply{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return protocol.ExecReply{}, err
	}
	req.Header.Set("Authorization", protocol.AuthorizationValue(key))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Timeout: limits.DialTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: limits.DialConnect}).DialContext,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	res, err := client.Do(req)
	if err != nil {
		return protocol.ExecReply{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, limits.DialReplyMax))
	if err != nil {
		return protocol.ExecReply{}, err
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode >= 500 {
		return protocol.ExecReply{}, errDial
	}
	if res.StatusCode != http.StatusOK {
		return protocol.ExecReply{Message: "命令端口拒绝"}, nil
	}
	var reply protocol.ExecReply
	if err := json.Unmarshal(body, &reply); err != nil {
		return protocol.ExecReply{}, err
	}
	return reply, nil
}
