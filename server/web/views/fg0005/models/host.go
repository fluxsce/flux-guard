package models

// HostRow 一台机器的当前占用，列表一行。
type HostRow struct {
	AgentID                string  `json:"agentId"`
	HostName               string  `json:"hostName"`
	ReportName             string  `json:"reportName"`
	IPAddress              string  `json:"ipAddress"`
	OSName                 string  `json:"osName"`
	CPUUsagePercent        float64 `json:"cpuUsagePercent"`
	LoadAvg1               float64 `json:"loadAvg1"`
	MemUsedPercent         float64 `json:"memUsedPercent"`
	MemUsedBytes           int64   `json:"memUsedBytes"`
	MemTotalBytes          int64   `json:"memTotalBytes"`
	DiskUsedPercent        float64 `json:"diskUsedPercent"`
	DiskReadBytesPerSec    float64 `json:"diskReadBytesPerSec"`
	DiskWriteBytesPerSec   float64 `json:"diskWriteBytesPerSec"`
	DiskUtilPercent        float64 `json:"diskUtilPercent"`
	NetReceiveBytesPerSec  float64 `json:"netReceiveBytesPerSec"`
	NetTransmitBytesPerSec float64 `json:"netTransmitBytesPerSec"`
	TCPEstablished         int64   `json:"tcpEstablished"`
	TCPTimeWait            int64   `json:"tcpTimeWait"`
	TCPCloseWait           int64   `json:"tcpCloseWait"`
	FDAllocated            int64   `json:"fdAllocated"`
	FDUsedPercent          float64 `json:"fdUsedPercent"`
	ProcCount              int64   `json:"procCount"`
	ZombieCount            int64   `json:"zombieCount"`
	ReportedTime           string  `json:"reportedTime"`
	OnlineFlag             string  `json:"onlineFlag"`
}

// DiskRow 一个挂载点的当前容量。
type DiskRow struct {
	MountPoint       string  `json:"mountPoint"`
	DeviceName       string  `json:"deviceName"`
	TotalBytes       int64   `json:"totalBytes"`
	UsedBytes        int64   `json:"usedBytes"`
	FreeBytes        int64   `json:"freeBytes"`
	UsedPercent      float64 `json:"usedPercent"`
	InodeUsedPercent float64 `json:"inodeUsedPercent"`
	ObservedTime     string  `json:"observedTime"`
}

// DiskIORow 一块磁盘的当前 IO。
type DiskIORow struct {
	DeviceName       string  `json:"deviceName"`
	ReadBytesPerSec  float64 `json:"readBytesPerSec"`
	WriteBytesPerSec float64 `json:"writeBytesPerSec"`
	ReadLatencyMs    float64 `json:"readLatencyMs"`
	WriteLatencyMs   float64 `json:"writeLatencyMs"`
	UtilPercent      float64 `json:"utilPercent"`
	ObservedTime     string  `json:"observedTime"`
}

// NetRow 一块网卡的当前流量。
type NetRow struct {
	NicName             string  `json:"nicName"`
	ReceiveBytesPerSec  float64 `json:"receiveBytesPerSec"`
	TransmitBytesPerSec float64 `json:"transmitBytesPerSec"`
	ReceiveErrs         int64   `json:"receiveErrs"`
	TransmitErrs        int64   `json:"transmitErrs"`
	ReceiveDrop         int64   `json:"receiveDrop"`
	TransmitDrop        int64   `json:"transmitDrop"`
	ObservedTime        string  `json:"observedTime"`
}

// CoreRow 一颗逻辑 CPU 的当前占用。
type CoreRow struct {
	CoreName     string  `json:"coreName"`
	UsagePercent float64 `json:"usagePercent"`
}

// ProcRow 一个占用靠前的进程。
type ProcRow struct {
	ProcessID    string  `json:"processId"`
	ProcessName  string  `json:"processName"`
	RSSBytes     int64   `json:"rssBytes"`
	CPUPercent   float64 `json:"cpuPercent"`
	ObservedTime string  `json:"observedTime"`
}

// ProcIORow 一个读写靠前的进程。速率是这一采集间隔里实际落到存储的字节。
type ProcIORow struct {
	ProcessID        string  `json:"processId"`
	ProcessName      string  `json:"processName"`
	ReadBytesPerSec  float64 `json:"readBytesPerSec"`
	WriteBytesPerSec float64 `json:"writeBytesPerSec"`
	ObservedTime     string  `json:"observedTime"`
}

// TcpRow 一台机器的当前传输层连接。
type TcpRow struct {
	EstablishedCount int64 `json:"establishedCount"`
	ListenCount      int64 `json:"listenCount"`
	TimeWaitCount    int64 `json:"timeWaitCount"`
	CloseWaitCount   int64 `json:"closeWaitCount"`
	SynSentCount     int64 `json:"synSentCount"`
	SynRecvCount     int64 `json:"synRecvCount"`
	FinWaitCount     int64 `json:"finWaitCount"`
	LastAckCount     int64 `json:"lastAckCount"`
	ClosingCount     int64 `json:"closingCount"`
	TCPInUseCount    int64 `json:"tcpInUseCount"`
	UDPInUseCount    int64 `json:"udpInUseCount"`
	RetransmitCount  int64 `json:"retransmitCount"`
}

// FdRow 一台机器的当前文件句柄。
type FdRow struct {
	AllocatedCount int64   `json:"allocatedCount"`
	MaximumCount   int64   `json:"maximumCount"`
	UsedPercent    float64 `json:"usedPercent"`
}

// CurvePoint 曲线上的一个点。
type CurvePoint struct {
	ObservedTime string  `json:"observedTime"`
	Value        float64 `json:"value"`
}

// Curve 一条可绘制的时间序列。Group 相同的曲线画在同一张图里，用来区分多块盘。
type Curve struct {
	Name   string       `json:"name"`
	Unit   string       `json:"unit"`
	Group  string       `json:"group,omitempty"`
	Points []CurvePoint `json:"points"`
}

// HostSeries 点开一台机器后看到的近期曲线。
type HostSeries struct {
	Curves []Curve `json:"curves"`
}

// HostAlert 是这台机器上尚未恢复的告警。
type HostAlert struct {
	IncidentID     string  `json:"incidentId"`
	RuleName       string  `json:"ruleName"`
	MetricCode     string  `json:"metricCode"`
	LevelCode      string  `json:"levelCode"`
	StatusCode     string  `json:"statusCode"`
	CurrentValue   float64 `json:"currentValue"`
	ThresholdValue float64 `json:"thresholdValue"`
	CompareOp      string  `json:"compareOp"`
	FirstFireTime  string  `json:"firstFireTime"`
}

// HostDetail 点开一台机器后看到的分类当前值。
type HostDetail struct {
	Disks  []DiskRow   `json:"disks"`
	DiskIO []DiskIORow `json:"diskIo"`
	Nets   []NetRow    `json:"nets"`
	Cores  []CoreRow   `json:"cores"`
	Procs  []ProcRow   `json:"procs"`
	ProcIO []ProcIORow `json:"procIos"`
	TCP    TcpRow      `json:"tcp"`
	FD     FdRow       `json:"fd"`
	Alerts []HostAlert `json:"alerts"`
}
