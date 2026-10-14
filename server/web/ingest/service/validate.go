package service

import (
	"errors"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"fluxguard/protocol"
)

var (
	errBatchID  = errors.New("批次号须为 8 到 64 位字母、数字、下划线或连字符")
	errAgent    = errors.New("代理种类无效")
	errEmpty    = errors.New("批次里没有样本或事件")
	errHost     = errors.New("主机名不能为空")
	errTooMany  = errors.New("单批样本或事件不能超过 1000 条")
	errMetric   = errors.New("指标名不能为空")
	errValue    = errors.New("指标值无效")
	errEvent    = errors.New("事件名不能为空")
	errSeverity = errors.New("事件级别只允许 INFO、WARN、ERROR")
	errLabel    = errors.New("标签过多或过长")
	errAppSide  = errors.New("应用侧资料无效")
	errGo       = errors.New("Go 运行时资料无效")
)

var (
	batchIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)
)

func validate(batch protocol.Batch) error {
	if batch.TenantID == "" || batch.AgentID == "" || !batchIDPattern.MatchString(batch.BatchID) {
		return errBatchID
	}
	if !protocol.KnownAgent(batch.AgentCode) {
		return errAgent
	}
	if len(batch.Samples) == 0 && len(batch.Events) == 0 && batch.Host == nil && batch.Java == nil && batch.Go == nil && len(batch.Sides()) == 0 {
		return errEmpty
	}
	if batch.Host != nil {
		name := strings.TrimSpace(batch.Host.HostName)
		if name == "" || len(name) > 100 || len(batch.Host.OSName) > 64 || len(batch.Host.ArchName) > 32 {
			return errHost
		}
		if utf8.RuneCountInString(strings.TrimSpace(batch.Host.ReportName)) > 100 {
			return errHost
		}
		host := batch.Host
		if len(host.MachineID) > 64 || len(host.KernelVersion) > 100 || len(host.AgentVersion) > 64 || len(host.IPList) > 255 {
			return errHost
		}
	}
	sides := batch.Sides()
	if len(sides) > 100 {
		return errAppSide
	}
	for i := range sides {
		if err := checkAppSide(&sides[i]); err != nil {
			return err
		}
	}
	if err := checkGo(batch.Go); err != nil {
		return err
	}
	if len(batch.Samples) > 1000 || len(batch.Events) > 1000 {
		return errTooMany
	}
	for _, sample := range batch.Samples {
		name := strings.TrimSpace(sample.MetricName)
		if name == "" || len(name) > 128 {
			return errMetric
		}
		if math.IsNaN(sample.MetricValue) || math.IsInf(sample.MetricValue, 0) {
			return errValue
		}
		if err := checkLabels(sample.Labels); err != nil {
			return err
		}
	}
	for _, event := range batch.Events {
		if strings.TrimSpace(event.EventName) == "" || len(event.EventName) > 128 {
			return errEvent
		}
		switch event.Severity {
		case "", "INFO", "WARN", "ERROR":
		default:
			return errSeverity
		}
		if len(event.MessageText) > 4000 {
			return errEvent
		}
		if err := checkLabels(event.Labels); err != nil {
			return err
		}
	}
	return nil
}

func checkGo(item *protocol.Go) error {
	if item == nil {
		return nil
	}
	if utf8.RuneCountInString(strings.TrimSpace(item.AppName)) > 100 {
		return errGo
	}
	if len(item.GoVersion) > 64 || len(item.ProcessID) > 32 || len(item.HostName) > 100 {
		return errGo
	}
	if item.NumCPU < 0 || item.NumCPU > 4096 || item.GoroutineCount < 0 {
		return errGo
	}
	if math.IsNaN(item.GCPauseMs) || math.IsInf(item.GCPauseMs, 0) || item.GCPauseMs < 0 {
		return errGo
	}
	return nil
}

func checkAppSide(side *protocol.AppSide) error {
	if side == nil {
		return nil
	}
	name := strings.TrimSpace(side.InstanceName)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return errAppSide
	}
	switch strings.TrimSpace(side.RuntimeKind) {
	case "systemd", "windows", "docker", "command":
	default:
		return errAppSide
	}
	switch strings.TrimSpace(side.Mode) {
	case "tomcat", "node", "script", "jar", "static", "files":
	default:
		return errAppSide
	}
	switch strings.TrimSpace(side.RunningFlag) {
	case "Y", "N":
	default:
		return errAppSide
	}
	if len(side.Artifacts) > 200 {
		return errAppSide
	}
	for _, item := range side.Artifacts {
		app := strings.TrimSpace(item.AppName)
		if app == "" || utf8.RuneCountInString(app) > 100 {
			return errAppSide
		}
		switch strings.TrimSpace(item.Kind) {
		case "war", "jar", "static", "node", "script", "runtime", "dir", "log":
		default:
			return errAppSide
		}
	}
	return nil
}

func checkLabels(labels map[string]string) error {
	if len(labels) > 16 {
		return errLabel
	}
	for key, value := range labels {
		if key == "" || len(key) > 64 || len(value) > 128 {
			return errLabel
		}
	}
	return nil
}
