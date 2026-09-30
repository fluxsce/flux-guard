package models

// JavaRow 一个 Java 进程的当前占用，列表一行。
type JavaRow struct {
	AgentID          string   `json:"agentId"`
	AppName          string   `json:"appName"`
	MainClass        string   `json:"mainClass"`
	ProcessID        string   `json:"processId"`
	JavaVersion      string   `json:"javaVersion"`
	StartTime        string   `json:"startTime"`
	HostName         string   `json:"hostName"`
	IPAddress        string   `json:"ipAddress"`
	HeapUsedPercent  float64  `json:"heapUsedPercent"`
	HeapUsedBytes    int64    `json:"heapUsedBytes"`
	HeapMaxBytes     int64    `json:"heapMaxBytes"`
	NonHeapUsedBytes int64    `json:"nonHeapUsedBytes"`
	ThreadCount      int64    `json:"threadCount"`
	DaemonCount      int64    `json:"daemonCount"`
	CollectionCount  int64    `json:"collectionCount"`
	CollectionTimeMs int64    `json:"collectionTimeMs"`
	ReportedTime     string   `json:"reportedTime"`
	Findings         []string `json:"findings"`
	FindingLevel     string   `json:"findingLevel"`
}

// GCRow 一个回收器的当前累计值。
type GCRow struct {
	CollectorName    string `json:"collectorName"`
	CollectionCount  int64  `json:"collectionCount"`
	CollectionTimeMs int64  `json:"collectionTimeMs"`
	ObservedTime     string `json:"observedTime"`
}

// JavaDetail 点开一个进程后看到的判断和当前回收器。
type JavaDetail struct {
	Findings     []string `json:"findings"`
	FindingLevel string   `json:"findingLevel"`
	Collectors   []GCRow  `json:"collectors"`
}

// CurvePoint 曲线上的一个点。
type CurvePoint struct {
	ObservedTime string  `json:"observedTime"`
	Value        float64 `json:"value"`
}

// Curve 一条可绘制的时间序列。Group 相同的曲线画在同一张图里。
type Curve struct {
	Name   string       `json:"name"`
	Unit   string       `json:"unit"`
	Group  string       `json:"group,omitempty"`
	Points []CurvePoint `json:"points"`
}

// JavaSeries 点开一个进程后看到的近期曲线。
type JavaSeries struct {
	Curves []Curve `json:"curves"`
}
