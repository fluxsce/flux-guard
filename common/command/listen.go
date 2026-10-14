// Package command 是 Go 代理的命令通道。
// 它监听 server 拨进来的请求，并在拨不通时自己去长轮询。采集不在这里。
package command

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"fluxguard/common/logger"
	"fluxguard/protocol"
)

// Exec 执行一条已通过密钥校验的命令。返回的文字写入命令结果。
type Exec func(ctx context.Context, call protocol.CommandCall) (string, error)

// Config 是命令通道配置。上报、取令、回执和命令端口都校验同一把代理令牌。
type Config struct {
	ServerURL  string
	AgentToken string
	AgentCode  string
	MachineID  string
	Listen     string
}

// Run 先按配置决定是否监听，然后一直长轮询，直到 ctx 取消。
// 监听地址和代理令牌必须同时有值，否则不打开端口。
func Run(ctx context.Context, cfg Config, exec Exec, onListen func(addr string)) {
	cfg.Listen = strings.TrimSpace(cfg.Listen)
	cfg.AgentToken = strings.TrimSpace(cfg.AgentToken)
	if cfg.Listen != "" && cfg.AgentToken == "" {
		logger.Warn(ctx, "command: 未配置 agent_token，不开启监听")
	}
	if cfg.Listen != "" && cfg.AgentToken != "" {
		go serve(ctx, cfg, exec, onListen)
	}
	poll(ctx, cfg, exec)
}

// Handler 是命令端口。缺少或错误的代理令牌一律拒绝，不执行动作。
func Handler(token string, exec Exec) http.Handler {
	token = strings.TrimSpace(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !allowKey(r.Header.Get("Authorization"), token) {
			writeReply(w, http.StatusUnauthorized, protocol.ExecReply{Message: "代理令牌无效"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var call protocol.CommandCall
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil || call.CommandID == "" || call.Action == "" {
			writeReply(w, http.StatusBadRequest, protocol.ExecReply{Message: "命令不完整"})
			return
		}
		message, err := exec(r.Context(), call)
		if err != nil {
			writeReply(w, http.StatusOK, protocol.ExecReply{Message: err.Error()})
			return
		}
		writeReply(w, http.StatusOK, protocol.ExecReply{OK: true, Message: message})
	})
}

// Advertise 把实际绑定地址收成可上报的 host:port。
// 端口必须是已绑定的端口；未写明网卡时用本机可达地址，避免上报 0.0.0.0 或端口 0。
func Advertise(listen string) string {
	host, port, err := net.SplitHostPort(strings.TrimSpace(listen))
	if err != nil || port == "" || port == "0" {
		return ""
	}
	ip := net.ParseIP(host)
	if host == "" || ip == nil || ip.IsUnspecified() {
		host = reachableIPv4()
		if host == "" {
			host = "127.0.0.1"
		}
	}
	return net.JoinHostPort(host, port)
}

func reachableIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.IsLoopback() {
				continue
			}
			if v4 := ipNet.IP.To4(); v4 != nil {
				return v4.String()
			}
		}
	}
	return ""
}

func allowKey(header, key string) bool {
	if key == "" {
		return false
	}
	got := protocol.BearerToken(header)
	if len(got) != len(key) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(key)) == 1
}

func writeReply(w http.ResponseWriter, status int, reply protocol.ExecReply) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(reply)
}

func serve(ctx context.Context, cfg Config, exec Exec, onListen func(addr string)) {
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		logger.Error(ctx, "command: 监听 %s: %v", cfg.Listen, err)
		return
	}
	if onListen != nil {
		if addr := Advertise(ln.Addr().String()); addr != "" {
			onListen(addr)
		}
	}
	srv := &http.Server{
		Handler: Handler(cfg.AgentToken, exec),
		// 与 server/utils/const/limits.HTTPReadHeader 保持一致（common 不依赖 server）。
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shut)
	}()
	err = srv.Serve(ln)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error(ctx, "command: 监听结束: %v", err)
	}
}
