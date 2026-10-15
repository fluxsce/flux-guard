// Package fetch 编排应用制品「代下」：能拨则中央直接拉进暂存；拨不通则下发出站推送命令。
// 与 release（换包）分开，避免 release 包继续膨胀。
package fetch

import (
	"context"
	"errors"
	"io"
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
	JobID         string `json:"jobId"`
	InstID        string `json:"instId"`
	FileName      string `json:"fileName"`
	Status        string `json:"status"` // waiting | ready | failed
	Message       string `json:"message,omitempty"`
	Mode          string `json:"mode,omitempty"` // dial | push
	BytesReceived int64  `json:"bytesReceived,omitempty"`
	BytesTotal    int64  `json:"bytesTotal,omitempty"`
	Phase         string `json:"phase,omitempty"` // queued | transferring | ready | failed
}

// Request 发起代下：优先短拨（异步拉流）；失败则出站推送。
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

	// 短拨探活+开流用独立超时；成功后后台拷贝，避免阻塞控制台请求超过客户端超时。
	appfile.PurgeStale()
	pullCtx, cancel := context.WithTimeout(context.Background(), limits.AppFileDownloadTimeout)
	fileName, body, size, dialErr := svc.PullAppsideFile(pullCtx, tenantID, inst.AgentID, path)
	if dialErr == nil {
		if fileName != "" {
			name = fileName
		}
		if size < 0 {
			size = 0
		}
		if size > limits.AppFileMaxBytes {
			cancel()
			body.Close()
			return Job{}, appfile.ErrTooLarge
		}
		jobID := id.New()
		if err := writeMeta(tenantID, jobID, name); err != nil {
			cancel()
			body.Close()
			return Job{}, err
		}
		_ = appfile.WriteProgress(tenantID, jobID, appfile.Progress{
			BytesReceived: 0,
			BytesTotal:    size,
			Status:        "transferring",
			Message:       "短拨拉取中",
		})
		go runDialSave(cancel, body, tenantID, jobID, size)
		return Job{
			JobID: jobID, InstID: instID, FileName: name,
			Status: "waiting", Mode: "dial", Phase: "transferring",
			BytesTotal: size,
		}, nil
	}
	cancel()

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
	_ = appfile.WriteProgress(tenantID, view.CommandID, appfile.Progress{
		Status:  "transferring",
		Message: "等待代理出站推送",
	})
	return Job{
		JobID: view.CommandID, InstID: instID, FileName: name,
		Status: mapCmd(view.Status), Message: view.Message, Mode: "push",
		Phase: "queued",
	}, nil
}

func runDialSave(cancel context.CancelFunc, body io.ReadCloser, tenantID, jobID string, size int64) {
	defer cancel()
	defer body.Close()
	if err := appfile.SaveWithProgress(tenantID, jobID, body, size); err != nil {
		_ = appfile.WriteProgress(tenantID, jobID, appfile.Progress{
			Status: "failed", Message: err.Error(), BytesTotal: size,
		})
	}
}

// Status 查任务是否已落盘，并附带字节进度。
func Status(ctx context.Context, db *dbx.DB, tenantID, jobID string) (Job, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return Job{}, errors.New("缺少任务")
	}
	out := Job{JobID: jobID, FileName: readMeta(tenantID, jobID)}
	prog := appfile.ReadProgress(tenantID, jobID)
	out.BytesReceived = prog.BytesReceived
	out.BytesTotal = prog.BytesTotal
	out.Phase = prog.Status
	if prog.Message != "" {
		out.Message = prog.Message
	}

	if appfile.Ready(tenantID, jobID) || prog.Status == "ready" {
		out.Status = "ready"
		out.Phase = "ready"
		if out.BytesReceived == 0 {
			out.BytesReceived = appfile.Size(tenantID, jobID)
		}
		if out.BytesTotal == 0 {
			out.BytesTotal = out.BytesReceived
		}
		return out, nil
	}
	if prog.Status == "failed" {
		out.Status = "failed"
		out.Phase = "failed"
		if out.Message == "" {
			out.Message = "代下失败"
		}
		return out, nil
	}

	svc := command.Current()
	if svc == nil {
		return Job{}, errors.New("命令通道未启用")
	}
	view, err := svc.Get(ctx, tenantID, jobID)
	if err != nil {
		// 短拨任务无命令行：仍在传输则 waiting，否则失败。
		if prog.Status == "transferring" {
			out.Status = "waiting"
			out.Phase = "transferring"
			if out.Message == "" {
				out.Message = "短拨拉取中"
			}
			return out, nil
		}
		out.Status = "failed"
		out.Phase = "failed"
		out.Message = "文件尚未就绪"
		return out, nil
	}
	if out.Message == "" {
		out.Message = view.Message
	}
	out.Status = mapCmd(view.Status)
	if view.Status == "done" && appfile.Ready(tenantID, jobID) {
		out.Status = "ready"
		out.Phase = "ready"
	} else if view.Status == "done" {
		out.Status = "failed"
		out.Phase = "failed"
		if out.Message == "" {
			out.Message = "代理已回执但文件未到达"
		}
	} else if view.Status == "failed" {
		out.Status = "failed"
		out.Phase = "failed"
	} else {
		out.Status = "waiting"
		if prog.BytesReceived > 0 {
			out.Phase = "transferring"
		} else {
			out.Phase = "queued"
			if out.Message == "" {
				out.Message = "等待代理出站推送"
			}
		}
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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
