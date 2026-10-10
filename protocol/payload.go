// Package protocol 是代理、外置组件进程和服务器之间的通讯协议。
// 它定义上报的字段、路径、鉴权头，以及外置插件的长度前缀信封和跟踪号。插件清单在 plugins/manifests。不加载扩展，也不是一个进程。
package protocol

import "time"

// Sample 一条数值监控样本。
type Sample struct {
	MetricName  string            `json:"metricName"`
	MetricValue float64           `json:"metricValue"`
	Labels      map[string]string `json:"labels,omitempty"`
	ObservedAt  time.Time         `json:"observedAt"`
}

// Event 一条非数值事件。
type Event struct {
	EventName   string            `json:"eventName"`
	Severity    string            `json:"severity,omitempty"`
	MessageText string            `json:"messageText,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	ObservedAt  time.Time         `json:"observedAt"`
}

// Host 是主机代理看到的当前机器资料。它不是曲线点，一台机器保留最新一份。
// MachineID 是操作系统的稳定机器编号，服务器用它把同一租户下的多台主机分成多行。
type Host struct {
	HostName       string    `json:"hostName"`
	OSName         string    `json:"osName"`
	OSVersion      string    `json:"osVersion,omitempty"`
	ArchName       string    `json:"archName"`
	CPUCount       int       `json:"cpuCount"`
	MemTotalBytes  uint64    `json:"memTotalBytes"`
	IPAddress      string    `json:"ipAddress,omitempty"`
	IPList         string    `json:"ipList,omitempty"`
	BootTime       time.Time `json:"bootTime,omitempty"`
	ReportName     string    `json:"reportName,omitempty"`
	MachineID      string    `json:"machineId,omitempty"`
	KernelVersion  string    `json:"kernelVersion,omitempty"`
	AgentVersion   string    `json:"agentVersion,omitempty"`
	CommandAddress string    `json:"commandAddress,omitempty"`
}

// Java 是 Java 代理看到的当前进程资料。它不是曲线点，一个进程保留最新一份。
type Java struct {
	AppName     string    `json:"appName,omitempty"`
	MainClass   string    `json:"mainClass,omitempty"`
	ProcessID   string    `json:"processId,omitempty"`
	JavaVersion string    `json:"javaVersion,omitempty"`
	StartTime   time.Time `json:"startTime,omitempty"`
	HostName    string    `json:"hostName,omitempty"`
	IPAddress   string    `json:"ipAddress,omitempty"`
}

// CollectResult 一个代理这一轮采集的产出，上报时放进 Request。
type CollectResult struct {
	Samples []Sample `json:"samples,omitempty"`
	Events  []Event  `json:"events,omitempty"`
	Host    *Host    `json:"host,omitempty"`
	Java    *Java    `json:"java,omitempty"`
}

// Request 是代理 POST 的 JSON 正文。租户和代理由令牌决定，不写进正文。
type Request struct {
	BatchID   string    `json:"batchId"`
	AgentCode string    `json:"agentCode"`
	SentAt    time.Time `json:"sentAt"`
	Samples   []Sample  `json:"samples,omitempty"`
	Events    []Event   `json:"events,omitempty"`
	Host      *Host     `json:"host,omitempty"`
	Java      *Java     `json:"java,omitempty"`
}

// Batch 把 Request 补上服务器从令牌认定的租户和代理。转发组件读的是这一份。
func (r Request) Batch(tenantID, agentID string) Batch {
	return Batch{
		TenantID:  tenantID,
		AgentID:   agentID,
		BatchID:   r.BatchID,
		AgentCode: r.AgentCode,
		SentAt:    r.SentAt,
		Samples:   r.Samples,
		Events:    r.Events,
		Host:      r.Host,
		Java:      r.Java,
	}
}

// Batch 是服务器认定身份之后的一批上报。
type Batch struct {
	TenantID  string
	AgentID   string
	BatchID   string
	AgentCode string
	SentAt    time.Time
	Samples   []Sample
	Events    []Event
	Host      *Host
	Java      *Java
}

// Result 是上报接口的成功应答。
type Result struct {
	Duplicate   bool `json:"duplicate"`
	SampleCount int  `json:"sampleCount"`
	EventCount  int  `json:"eventCount"`
}
