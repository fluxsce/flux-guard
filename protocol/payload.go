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
	Description    string    `json:"description,omitempty"` // 代理描述，写入 FG_AGENT.noteText
	MachineID      string    `json:"machineId,omitempty"`
	KernelVersion  string    `json:"kernelVersion,omitempty"`
	AgentVersion   string    `json:"agentVersion,omitempty"`
	CommandAddress string    `json:"commandAddress,omitempty"`
}

// Java 是 Java 代理看到的当前进程资料。它不是曲线点，一个实例保留最新一份。
// InstanceName 区分同一台机器上的同一服务。留空时用主机名或 Pod 名。
type Java struct {
	AppName      string    `json:"appName,omitempty"`
	InstanceName string    `json:"instanceName,omitempty"`
	MainClass    string    `json:"mainClass,omitempty"`
	ProcessID    string    `json:"processId,omitempty"`
	JavaVersion  string    `json:"javaVersion,omitempty"`
	StartTime    time.Time `json:"startTime,omitempty"`
	HostName     string    `json:"hostName,omitempty"`
	IPAddress    string    `json:"ipAddress,omitempty"`
}

// AppArtifact 是应用侧代理看到的一个可更新制品。
// Kind 只允许 war、jar、static、node、script。mode=files 的目录仓也按扩展名报 war/jar/static。
// WAR/JAR 版本来自文件名里的 ##，没有则为空。
type AppArtifact struct {
	AppName string `json:"appName"`
	Kind    string `json:"kind"`
	Version string `json:"version,omitempty"`
	Path    string `json:"path,omitempty"`
}

// AppSide 是应用侧代理看守的那一套运行时。它不是主机指标，也不是 Java 曲线。
// 应用进程退出后这一份仍然上报，RunningFlag 为 N。
type AppSide struct {
	InstanceName   string        `json:"instanceName"`
	RuntimeKind    string        `json:"runtimeKind"`
	Mode           string        `json:"mode"`
	HomePath       string        `json:"homePath,omitempty"` // 兼容旧上报，新代理不再填写
	BasePath       string        `json:"basePath,omitempty"`
	TomcatVersion  string        `json:"tomcatVersion,omitempty"`
	CommandAddress string        `json:"commandAddress,omitempty"`
	HostName       string        `json:"hostName,omitempty"`
	IPAddress      string        `json:"ipAddress,omitempty"`
	MachineID      string        `json:"machineId,omitempty"`
	Description    string        `json:"description,omitempty"` // 运行时描述，写入 FG_APP_INST.noteText
	RunningFlag    string        `json:"runningFlag"`
	Artifacts      []AppArtifact `json:"artifacts,omitempty"`
}

// Go 是 Go 进程看到的自己。它不是应用清单，进程退出后这一行不再更新。
type Go struct {
	AppName        string    `json:"appName,omitempty"`
	GoVersion      string    `json:"goVersion,omitempty"`
	ProcessID      string    `json:"processId,omitempty"`
	StartTime      time.Time `json:"startTime,omitempty"`
	HostName       string    `json:"hostName,omitempty"`
	NumCPU         int       `json:"numCpu,omitempty"`
	GoroutineCount int       `json:"goroutineCount,omitempty"`
	HeapAllocBytes uint64    `json:"heapAllocBytes,omitempty"`
	HeapSysBytes   uint64    `json:"heapSysBytes,omitempty"`
	HeapObjects    uint64    `json:"heapObjects,omitempty"`
	GCCount        uint32    `json:"gcCount,omitempty"`
	GCPauseMs      float64   `json:"gcPauseMs,omitempty"`
}

// CollectResult 一个代理这一轮采集的产出，上报时放进 Request。
type CollectResult struct {
	Samples  []Sample  `json:"samples,omitempty"`
	Events   []Event   `json:"events,omitempty"`
	Host     *Host     `json:"host,omitempty"`
	Java     *Java     `json:"java,omitempty"`
	Go       *Go       `json:"go,omitempty"`
	AppSide  *AppSide  `json:"appSide,omitempty"`
	AppSides []AppSide `json:"appSides,omitempty"`
}

// Request 是代理 POST 的 JSON 正文。租户和代理由令牌决定，不写进正文。
type Request struct {
	BatchID      string    `json:"batchId"`
	AgentCode    string    `json:"agentCode"`
	AgentVersion string    `json:"agentVersion,omitempty"`
	GoOS         string    `json:"goos,omitempty"`
	GoArch       string    `json:"goarch,omitempty"`
	Description  string    `json:"description,omitempty"` // 代理描述（configs 的 description），写入 FG_AGENT.noteText
	SentAt       time.Time `json:"sentAt"`
	Samples      []Sample  `json:"samples,omitempty"`
	Events       []Event   `json:"events,omitempty"`
	Host         *Host     `json:"host,omitempty"`
	Java         *Java     `json:"java,omitempty"`
	Go           *Go       `json:"go,omitempty"`
	AppSide      *AppSide  `json:"appSide,omitempty"`
	AppSides     []AppSide `json:"appSides,omitempty"`
}

// Batch 把 Request 补上服务器从令牌认定的租户和代理。转发组件读的是这一份。
func (r Request) Batch(tenantID, agentID string) Batch {
	return Batch{
		TenantID:     tenantID,
		AgentID:      agentID,
		BatchID:      r.BatchID,
		AgentCode:    r.AgentCode,
		AgentVersion: r.AgentVersion,
		GoOS:         r.GoOS,
		GoArch:       r.GoArch,
		SentAt:       r.SentAt,
		Samples:      r.Samples,
		Events:       r.Events,
		Host:         r.Host,
		Java:         r.Java,
		Go:           r.Go,
		AppSide:      r.AppSide,
		AppSides:     r.AppSides,
	}
}

// Batch 是服务器认定身份之后的一批上报。
type Batch struct {
	TenantID     string
	AgentID      string
	BatchID      string
	AgentCode    string
	AgentVersion string
	GoOS         string
	GoArch       string
	SentAt       time.Time
	Samples      []Sample
	Events       []Event
	Host         *Host
	Java         *Java
	Go           *Go
	AppSide      *AppSide
	AppSides     []AppSide
}

// Sides 返回这一批里的全部运行时。单套放在 AppSide，一台机器的多套放在 AppSides。
func (b Batch) Sides() []AppSide {
	out := make([]AppSide, 0, len(b.AppSides)+1)
	if b.AppSide != nil {
		out = append(out, *b.AppSide)
	}
	out = append(out, b.AppSides...)
	return out
}

// Result 是上报接口的成功应答。
type Result struct {
	Duplicate   bool `json:"duplicate"`
	SampleCount int  `json:"sampleCount"`
	EventCount  int  `json:"eventCount"`
}
