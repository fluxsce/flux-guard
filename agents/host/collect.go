// Package host 是主机代理：采集这台机器的资料和主机指标，再自行上报。
package host

import (
	"context"
	"time"

	"fluxguard/protocol"
)

// Code 是协议里的代理种类。
const Code = protocol.AgentHost

// Collect 读取操作系统看到的主机资源。
// 使用率、磁盘 IO 速率和网卡速率从上一轮累计值算出，第一轮只有累计值。
func Collect(ctx context.Context) (protocol.CollectResult, error) {
	if err := ctx.Err(); err != nil {
		return protocol.CollectResult{}, err
	}
	now := time.Now().UTC()
	samples, memTotal, err := sample(ctx, now)
	if err != nil {
		return protocol.CollectResult{}, err
	}
	info := identity(memTotal)
	return protocol.CollectResult{Host: &info, Samples: samples}, nil
}
