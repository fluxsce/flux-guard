package files

import (
	"context"
	"fluxguard/agents/appside/core"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fluxguard/protocol"
)

const filesMaxDepth = 8

func init() {
	core.RegisterMode(filesMode{})
}

type filesMode struct{}

func (filesMode) Name() string { return "files" }

func (filesMode) AllowKind(kind string) bool {
	switch kind {
	case "war", "jar", "static":
		return true
	default:
		return false
	}
}

func (filesMode) NeedsRestart(kind string) bool { return false }

func (filesMode) SupportUpgrade() bool { return false }

func (filesMode) FillSide(side *protocol.AppSide, rt core.Runtime) {}

func (filesMode) CollectArtifacts(ctx context.Context, rt core.Runtime) ([]core.Artifact, error) {
	_ = ctx
	// 心跳只报站点标记；子文件在控制台打开时短拨列目录。
	return []core.Artifact{{
		AppName: rt.Instance,
		Kind:    "core.Runtime",
		Path:    rt.Base,
	}}, nil
}

func (filesMode) ListInventory(rt core.Runtime) ([]core.Artifact, error) {
	return ListFiles(rt.Base)
}

func (filesMode) Replace(rt core.Runtime, call protocol.CommandCall, file string) (core.ApplyReport, error) {
	return core.ReplaceFile(rt.Base, call.AppName, file, call.FileName, rt.BackupDir, rt.Instance)
}

func (filesMode) Upgrade(rt core.Runtime, zipPath string) (core.ApplyReport, error) {
	return core.ApplyReport{}, fmt.Errorf("files 模式不支持运行时升级")
}

func (filesMode) MarkStartupLog(rt core.Runtime) core.LogMark { return core.LogMark{} }

func (filesMode) ProbeStartup(ctx context.Context, rt core.Runtime, mark core.LogMark) protocol.CommandStep {
	_ = ctx
	_ = rt
	_ = mark
	return protocol.CommandStep{Name: "startup", Status: "done", Detail: "files 模式无进程启动"}
}

// ListFiles 递归读取制品仓：子目录、war/jar/zip，以及常见日志文件。
func ListFiles(dir string) ([]core.Artifact, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	var out []core.Artifact
	seen := map[string]bool{}
	if err := walkFiles(abs, abs, 0, &out, seen); err != nil {
		return nil, err
	}
	return out, nil
}

func walkFiles(root, dir string, depth int, out *[]core.Artifact, seen map[string]bool) error {
	if depth > filesMaxDepth {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(dir, name)
		rel, err := filepath.Rel(root, full)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			app := rel
			if seen[app] {
				continue
			}
			if err := core.EnsureFilesListCap(len(*out)); err != nil {
				return err
			}
			seen[app] = true
			*out = append(*out, core.Artifact{AppName: app, Kind: "dir", Path: full})
			if err := walkFiles(root, full, depth+1, out, seen); err != nil {
				return err
			}
			continue
		}
		kind := core.FileKind(filepath.Ext(name))
		if kind == "" {
			continue
		}
		base, version := core.AppCode(name)
		if base == "" {
			continue
		}
		if kind == "war" && core.BuiltinWars[base] && !strings.Contains(rel, "/") {
			continue
		}
		app := base
		if parent := filepath.ToSlash(filepath.Dir(rel)); parent != "." && parent != "" {
			app = parent + "/" + base
		}
		if seen[app] {
			continue
		}
		if err := core.EnsureFilesListCap(len(*out)); err != nil {
			return err
		}
		seen[app] = true
		*out = append(*out, core.Artifact{
			AppName: app, Kind: kind, Version: version, Path: full,
		})
	}
	return nil
}
