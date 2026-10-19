package models

// JavaRow 一个 Java 实例的当前占用，列表一行。
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
	RSSBytes         int64    `json:"rssBytes"`
	ThreadCount      int64    `json:"threadCount"`
	DaemonCount      int64    `json:"daemonCount"`
	CollectionCount  int64    `json:"collectionCount"`
	CollectionTimeMs int64    `json:"collectionTimeMs"`
	ReportedTime     string   `json:"reportedTime"`
	OnlineFlag       string   `json:"onlineFlag"`
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

// JavaDetail 点开一个进程后看到的判断、最近一拍、当前代回收器和上一代结束时的累计。
type JavaDetail struct {
	Findings     []string        `json:"findings"`
	FindingLevel string          `json:"findingLevel"`
	Beat         JavaBeat        `json:"beat"`
	Collectors   []GCRow         `json:"collectors"`
	Previous     *JavaGeneration `json:"previous,omitempty"`
}

// JavaBeat 是最近一拍的开销、死锁和停顿。小于 0 表示没采到，不能当成 0。
type JavaBeat struct {
	OverheadPercent float64     `json:"overheadPercent"`
	OverheadTime    string      `json:"overheadTime,omitempty"`
	DeadlockCount   int64       `json:"deadlockCount"`
	DeadlockText    string      `json:"deadlockText,omitempty"`
	DeadlockTime    string      `json:"deadlockTime,omitempty"`
	Pauses          []JavaPause `json:"pauses"`
}

// JavaPause 一个回收器在最近一拍里的停顿和周期。
// 堆前后和原因只属于最长的那一次停顿。晋升是年轻代合计，小于 0 表示没测全。
type JavaPause struct {
	CollectorName   string `json:"collectorName"`
	PauseCount      int64  `json:"pauseCount"`
	PauseMaxMs      int64  `json:"pauseMaxMs"`
	PauseTotalMs    int64  `json:"pauseTotalMs"`
	PauseP50Ms      int64  `json:"pauseP50Ms"`
	PauseP99Ms      int64  `json:"pauseP99Ms"`
	HeapBeforeBytes int64  `json:"heapBeforeBytes"`
	HeapAfterBytes  int64  `json:"heapAfterBytes"`
	PromotedBytes   int64  `json:"promotedBytes"`
	PauseCause      string `json:"pauseCause,omitempty"`
	CycleCount      int64  `json:"cycleCount"`
	CycleTimeMs     int64  `json:"cycleTimeMs"`
	ObservedTime    string `json:"observedTime"`
}

// JavaGeneration 一次 JVM 启动到下一次重启。当前代没有 ClosedTime。
type JavaGeneration struct {
	ProcessID        string  `json:"processId"`
	StartTime        string  `json:"startTime"`
	OpenedTime       string  `json:"openedTime"`
	ClosedTime       string  `json:"closedTime,omitempty"`
	CollectionCount  int64   `json:"collectionCount"`
	CollectionTimeMs int64   `json:"collectionTimeMs"`
	Collectors       []GCRow `json:"collectors"`
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

// JavaRestart 是进程重启后第一拍的位置。曲线在这里断开。
type JavaRestart struct {
	ObservedTime string `json:"observedTime"`
	StartTime    string `json:"startTime,omitempty"`
	ProcessID    string `json:"processId,omitempty"`
}

// JavaSeries 点开一个进程后看到的近期曲线。Restarts 标出窗口内的重启。
type JavaSeries struct {
	Curves   []Curve       `json:"curves"`
	Restarts []JavaRestart `json:"restarts"`
}
