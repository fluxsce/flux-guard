package appside

import (
	"context"
	"net"
	"os"

	"fluxguard/agents/appside/core"
	"fluxguard/protocol"
)

// CollectAll 读取这一进程看守的全部套件。某一套失败时仍上报其它套，并带上空制品骨架。
func CollectAll(ctx context.Context, list []Runtime, commandAddr string) (protocol.CollectResult, error) {
	sides := make([]protocol.AppSide, 0, len(list))
	for _, rt := range list {
		one, err := Collect(ctx, rt, commandAddr)
		if err != nil {
			sides = append(sides, skeletonSide(rt, commandAddr))
			continue
		}
		if one.AppSide != nil {
			sides = append(sides, *one.AppSide)
		}
	}
	return protocol.CollectResult{AppSides: sides}, nil
}

// Collect 读取一套应用。应用进程不在时仍然返回清单，运行标记为 N。
func Collect(ctx context.Context, rt Runtime, commandAddr string) (protocol.CollectResult, error) {
	if err := ctx.Err(); err != nil {
		return protocol.CollectResult{}, err
	}
	m, err := core.LookupMode(rt.Mode)
	if err != nil {
		return protocol.CollectResult{}, err
	}
	hostName, _ := os.Hostname()
	running := "N"
	if Running(ctx, rt) {
		running = "Y"
	}
	side := &protocol.AppSide{
		InstanceName:   rt.Instance,
		RuntimeKind:    rt.Kind,
		Mode:           rt.Mode,
		BasePath:       rt.Base,
		CommandAddress: commandAddr,
		HostName:       hostName,
		IPAddress:      localIPv4(),
		Description:    rt.Description,
		RunningFlag:    running,
	}
	m.FillSide(side, rt)
	found, err := m.CollectArtifacts(ctx, rt)
	if err != nil {
		return protocol.CollectResult{}, err
	}
	side.Artifacts = core.ToProtocol(found)
	return protocol.CollectResult{AppSide: side}, nil
}

func skeletonSide(rt Runtime, commandAddr string) protocol.AppSide {
	hostName, _ := os.Hostname()
	return protocol.AppSide{
		InstanceName:   rt.Instance,
		RuntimeKind:    rt.Kind,
		Mode:           rt.Mode,
		BasePath:       rt.Base,
		CommandAddress: commandAddr,
		HostName:       hostName,
		IPAddress:      localIPv4(),
		Description:    rt.Description,
		RunningFlag:    "N",
		Artifacts:      nil,
	}
}

func localIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if ip := ipnet.IP.To4(); ip != nil {
			return ip.String()
		}
	}
	return ""
}
