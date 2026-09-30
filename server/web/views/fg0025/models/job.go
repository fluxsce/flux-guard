package models

import "fluxguard/server/job/ext/archive"

// JobRow 任务调度列表一行。
type JobRow struct {
	Code            string `json:"code"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Kind            string `json:"kind"`
	Source          string `json:"source"`
	Scope           string `json:"scope"`
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode"`
	IntervalMinutes int    `json:"intervalMinutes"`
	RunAt           string `json:"runAt"`
	Cron            string `json:"cron,omitempty"`
	TimeoutSeconds  int    `json:"timeoutSeconds"`
	RetryCount      int    `json:"retryCount"`
	Deletable       bool   `json:"deletable"`
	LastRunAt       string `json:"lastRunAt"`
	LastOK          bool   `json:"lastOk"`
	LastDurationMs  int64  `json:"lastDurationMs"`
	LastResult      string `json:"lastResult,omitempty"`
	LastError       string `json:"lastError,omitempty"`
	LastTrigger     string `json:"lastTrigger,omitempty"`
	NextStart       string `json:"nextStart,omitempty"`
	RanAtLeast      bool   `json:"ranAtLeast"`
}

// JobKind 可以用来新建任务的执行器种类。
type JobKind struct {
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// JobList 任务列表应答。
type JobList struct {
	Editable bool      `json:"editable"`
	Items    []JobRow  `json:"items"`
	Kinds    []JobKind `json:"kinds"`
}

// ArchiveJob 数据归档任务详情。
type ArchiveJob struct {
	Code            string `json:"code"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Kind            string `json:"kind"`
	Source          string `json:"source"`
	Scope           string `json:"scope"`
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode"`
	IntervalMinutes int    `json:"intervalMinutes"`
	RunAt           string `json:"runAt"`
	Cron            string `json:"cron"`
	TimeoutSeconds  int    `json:"timeoutSeconds"`
	RetryCount      int    `json:"retryCount"`
	AuditLogDays    int    `json:"auditLogDays"`
	OpsLogDays      int    `json:"opsLogDays"`
	AlertDays       int    `json:"alertDays"`
	SampleDays      int    `json:"sampleDays"`
	Params          string `json:"params,omitempty"`
	CurrentVersion  int    `json:"currentVersion"`
	Editable        bool   `json:"editable"`
	Deletable       bool   `json:"deletable"`
	LastRunAt       string `json:"lastRunAt"`
	LastOK          bool   `json:"lastOk"`
	LastDurationMs  int64  `json:"lastDurationMs"`
	LastResult      string `json:"lastResult,omitempty"`
	LastError       string `json:"lastError,omitempty"`
	LastTrigger     string `json:"lastTrigger,omitempty"`
	NextStart       string `json:"nextStart,omitempty"`
	RanAtLeast      bool   `json:"ranAtLeast"`
}

// SaveArchiveRequest 保存数据归档任务。
type SaveArchiveRequest struct {
	Enabled         *bool  `json:"enabled"`
	Mode            string `json:"mode"`
	IntervalMinutes int    `json:"intervalMinutes"`
	RunAt           string `json:"runAt"`
	Cron            string `json:"cron"`
	TimeoutSeconds  int    `json:"timeoutSeconds"`
	RetryCount      int    `json:"retryCount"`
	AuditLogDays    int    `json:"auditLogDays"`
	OpsLogDays      int    `json:"opsLogDays"`
	AlertDays       int    `json:"alertDays"`
	SampleDays      int    `json:"sampleDays"`
	CurrentVersion  int    `json:"currentVersion"`
}

// ArchiveFrom 把归档请求收成归档策略。
func ArchiveFrom(req SaveArchiveRequest) archive.Policy {
	v := archive.Policy{
		Enabled:         true,
		Mode:            req.Mode,
		IntervalMinutes: req.IntervalMinutes,
		RunAt:           req.RunAt,
		Cron:            req.Cron,
		TimeoutSeconds:  req.TimeoutSeconds,
		RetryCount:      req.RetryCount,
		Params: archive.Params{
			AuditLogDays: req.AuditLogDays,
			OpsLogDays:   req.OpsLogDays,
			AlertDays:    req.AlertDays,
			SampleDays:   req.SampleDays,
		},
	}
	if req.Enabled != nil {
		v.Enabled = *req.Enabled
	}
	return v
}

// SaveJobRequest 新建或修改一条任务。种类决定调用哪一个已登记实现。
type SaveJobRequest struct {
	Code            string `json:"code"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Kind            string `json:"kind"`
	Enabled         *bool  `json:"enabled"`
	Mode            string `json:"mode"`
	IntervalMinutes int    `json:"intervalMinutes"`
	RunAt           string `json:"runAt"`
	Cron            string `json:"cron"`
	TimeoutSeconds  int    `json:"timeoutSeconds"`
	RetryCount      int    `json:"retryCount"`
	AuditLogDays    int    `json:"auditLogDays"`
	OpsLogDays      int    `json:"opsLogDays"`
	AlertDays       int    `json:"alertDays"`
	SampleDays      int    `json:"sampleDays"`
	Params          string `json:"params"`
	CurrentVersion  int    `json:"currentVersion"`
}

// RunJobRequest 前端立即执行或删除。
type RunJobRequest struct {
	Code string `json:"code"`
}

// JobLogRow 一条执行日志。
type JobLogRow struct {
	LogID      string `json:"logId"`
	Trigger    string `json:"trigger"`
	StartTime  string `json:"startTime"`
	EndTime    string `json:"endTime"`
	DurationMs int64  `json:"durationMs"`
	OK         bool   `json:"ok"`
	Result     string `json:"result,omitempty"`
	ErrorText  string `json:"errorText,omitempty"`
}

// JobLogList 某任务的历史执行日志，新的在前。
type JobLogList struct {
	Code  string      `json:"code"`
	Title string      `json:"title"`
	Items []JobLogRow `json:"items"`
}
