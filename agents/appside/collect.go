package appside

import (
	"context"
	"net"
	"os"
	"path/filepath"

	"fluxguard/protocol"
)

// CollectAll 读取这一进程看守的全部套件。某一套应用进程不在时仍然返回，运行标记为 N。
func CollectAll(ctx context.Context, list []Runtime, commandAddr string) (protocol.CollectResult, error) {
	sides := make([]protocol.AppSide, 0, len(list))
	for _, rt := range list {
		one, err := Collect(ctx, rt, commandAddr)
		if err != nil {
			return protocol.CollectResult{}, err
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
		RunningFlag:    running,
	}
	switch rt.Mode {
	case "tomcat":
		side.TomcatVersion = TomcatVersion(rt.Base)
		found, err := ListWebapps(filepath.Join(rt.Base, "webapps"))
		if err != nil {
			return protocol.CollectResult{}, err
		}
		side.Artifacts = toProtocol(found)
	case "node":
		pkg := ReadNodePackage(rt.Base)
		side.Artifacts = []protocol.AppArtifact{{
			AppName: NodeAppName(rt),
			Kind:    "node",
			Version: pkg.Version,
			Path:    rt.Base,
		}}
	case "script":
		side.Artifacts = []protocol.AppArtifact{{
			AppName: rt.Instance,
			Kind:    "script",
			Path:    rt.Base,
		}}
	case "jar":
		name, version := AppCode(rt.Jar)
		if name != "" {
			side.Artifacts = []protocol.AppArtifact{{
				AppName: name,
				Kind:    "jar",
				Version: version,
				Path:    rt.Jar,
			}}
		}
	case "static":
		name := rt.StaticApp
		if name == "" {
			name = rt.Instance
		}
		side.Artifacts = []protocol.AppArtifact{{
			AppName: name,
			Kind:    "static",
			Path:    rt.StaticDir,
		}}
	}
	return protocol.CollectResult{AppSide: side}, nil
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

func toProtocol(in []Artifact) []protocol.AppArtifact {
	out := make([]protocol.AppArtifact, 0, len(in))
	for _, item := range in {
		out = append(out, protocol.AppArtifact{
			AppName: item.AppName,
			Kind:    item.Kind,
			Version: item.Version,
			Path:    item.Path,
		})
	}
	return out
}
