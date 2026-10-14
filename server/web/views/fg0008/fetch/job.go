// Package fetch 编排应用制品「代下」：能拨则中央直接拉进暂存；拨不通则下发出站推送命令。
// 与 release（换包）分开，避免 release 包继续膨胀。
package fetch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/command"
	"fluxguard/server/helper/appfile"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/web/views/fg0008/dao"
	dbx "github.com/fluxsce/dbx/db"
)

// Job 是一次代下任务。
type Job struct {
	JobID    string `json:"jobId"`
	InstID   string `json:"instId"`
	FileName string `json:"fileName"`
	Status   string `json:"status"` // waiting | ready | failed
	Message  string `json:"message,omitempty"`
	Mode     string `json:"mode,omitempty"` // dial | push
}

// Request 发起代下：优先短拨；失败则出站推送。
func Request(ctx context.Context, db *dbx.DB, tenantID, user, instID string) (Job, error) {
	instID = strings.TrimSpace(instID)
	if instID == "" {
		return Job{}, errors.New("请选择文件")
	}
	inst, err := dao.GetInstance(ctx, db, tenantID, instID)
	if err != nil {
		return Job{}, err
	}
	if strings.EqualFold(strings.TrimSpace(inst.ArtifactKind), "dir") {
		return Job{}, errors.New("不能下载目录，请进入后选文件")
	}
	path := strings.TrimSpace(inst.ArtifactPath)
	if path == "" {
		return Job{}, errors.New("该行没有制品路径")
	}
	name := filepath.Base(path)
	svc := command.Current()
	if svc == nil {
		return Job{}, errors.New("命令通道未启用")
	}

	// 短拨整段传输用 AppFileDownloadTimeout；8s 只适合探活，会在读 body 时被 cancel 截断大文件。
	pullCtx, cancel := context.WithTimeout(ctx, limits.AppFileDownloadTimeout)
	defer cancel()
	if fileName, body, _, dialErr := svc.PullAppsideFile(pullCtx, tenantID, inst.AgentID, path); dialErr == nil {
		defer body.Close()
		if fileName != "" {
			name = fileName
		}
		jobID := id.New()
		if err := appfile.Save(tenantID, jobID, body); err != nil {
			return Job{}, err
		}
		_ = writeMeta(tenantID, jobID, name)
		return Job{JobID: jobID, InstID: instID, FileName: name, Status: "ready", Mode: "dial"}, nil
	}

	view, err := svc.Dispatch(ctx, tenantID, user, command.DispatchInput{
		AgentID: inst.AgentID, Action: protocol.ActionAppFilePush,
		ReleaseID: "push", Version: "fetch", Instance: inst.InstanceName,
		AppName: inst.AppCode, Kind: inst.ArtifactKind,
		TargetPath: path, FileName: name,
	})
	if err != nil {
		return Job{}, err
	}
	_ = writeMeta(tenantID, view.CommandID, name)
	return Job{
		JobID: view.CommandID, InstID: instID, FileName: name,
		Status: mapCmd(view.Status), Message: view.Message, Mode: "push",
	}, nil
}

// Status 查任务是否已落盘。
func Status(ctx context.Context, db *dbx.DB, tenantID, jobID string) (Job, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return Job{}, errors.New("缺少任务")
	}
	out := Job{JobID: jobID, FileName: readMeta(tenantID, jobID)}
	if appfile.Ready(tenantID, jobID) {
		out.Status = "ready"
		return out, nil
	}
	svc := command.Current()
	if svc == nil {
		return Job{}, errors.New("命令通道未启用")
	}
	view, err := svc.Get(ctx, tenantID, jobID)
	if err != nil {
		// 拨号任务无命令行，暂存又没有 → 失败。
		out.Status = "failed"
		out.Message = "文件尚未就绪"
		return out, nil
	}
	out.Message = view.Message
	out.Status = mapCmd(view.Status)
	if view.Status == "done" && appfile.Ready(tenantID, jobID) {
		out.Status = "ready"
	} else if view.Status == "done" {
		out.Status = "failed"
		if out.Message == "" {
			out.Message = "代理已回执但文件未到达"
		}
	} else if view.Status == "failed" {
		out.Status = "failed"
	} else {
		out.Status = "waiting"
	}
	return out, nil
}

// Open 打开已就绪的暂存。
func Open(tenantID, jobID string) (*os.File, string, error) {
	if !appfile.Ready(tenantID, jobID) {
		return nil, "", errors.New("文件尚未就绪")
	}
	file, err := appfile.Open(tenantID, jobID)
	if err != nil {
		return nil, "", err
	}
	name := readMeta(tenantID, jobID)
	if name == "" {
		name = jobID
	}
	return file, name, nil
}

func mapCmd(status string) string {
	switch status {
	case "done":
		return "ready"
	case "failed":
		return "failed"
	default:
		return "waiting"
	}
}

func writeMeta(tenantID, jobID, name string) error {
	path, err := appfile.Path(tenantID, jobID)
	if err != nil {
		return err
	}
	return os.WriteFile(path+".name", []byte(strings.TrimSpace(name)), 0o644)
}

func readMeta(tenantID, jobID string) string {
	path, err := appfile.Path(tenantID, jobID)
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(path + ".name")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
