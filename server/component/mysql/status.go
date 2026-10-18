package mysql

import "time"

// Counters 是 SHOW GLOBAL STATUS 里的累计计数，用来和上一拍相减。
type Counters struct {
	Questions    int64 `json:"questions"`
	Slow         int64 `json:"slow"`
	Select       int64 `json:"select"`
	Insert       int64 `json:"insert"`
	Update       int64 `json:"update"`
	Delete       int64 `json:"delete"`
	BytesIn      int64 `json:"bytesIn"`
	BytesOut     int64 `json:"bytesOut"`
	Aborted      int64 `json:"aborted"`
	LockWaits    int64 `json:"lockWaits"`
	TmpTables    int64 `json:"tmpTables"`
	TmpDisk      int64 `json:"tmpDisk"`
	PoolReads    int64 `json:"poolReads"`
	PoolReadReqs int64 `json:"poolReadReqs"`
	Uptime       int64 `json:"uptime"`
}

// ReplicaInfo 是从库复制线程的当前状态。不是从库时为空。
type ReplicaInfo struct {
	IO       string `json:"io"`
	SQL      string `json:"sql"`
	Source   string `json:"source,omitempty"`
	IOError  string `json:"ioError,omitempty"`
	SQLError string `json:"sqlError,omitempty"`
}

// Session 是一条已执行至少 1 秒、且不是 Sleep 的会话。
type Session struct {
	ID      int64  `json:"id"`
	User    string `json:"user,omitempty"`
	Host    string `json:"host,omitempty"`
	DB      string `json:"db,omitempty"`
	Command string `json:"command,omitempty"`
	Time    int64  `json:"time"`
	State   string `json:"state,omitempty"`
	Info    string `json:"info,omitempty"`
}

// Sample 是一轮 MySQL 采集。未知的比率和速率为 -1，不用 0 表示没有数据。
type Sample struct {
	ThreadsConnected       int64
	ThreadsRunning         int64
	Questions              int64
	SlowQueries            int64
	UptimeSec              int64
	MaxConnections         int64
	BufferPoolHitRate      float64
	BufferPoolDirtyPercent float64
	QuestionsPerSec        float64
	SlowPerSec             float64
	SelectPerSec           float64
	InsertPerSec           float64
	UpdatePerSec           float64
	DeletePerSec           float64
	NetInKbps              float64
	NetOutKbps             float64
	ConnUsagePercent       float64
	AbortedPerSec          float64
	LockWaitsPerSec        float64
	TmpDiskPercent         float64
	ReplicaLagSec          float64
	ReplicaIO              float64
	ReplicaSQL             float64
	Version                string
	Role                   string
	Counters               Counters
	Replica                *ReplicaInfo
	Sessions               []Session
	SessionNote            string
	PoolBytes              int64
}

// Prior 是上一拍的累计计数和观测时间。
type Prior struct {
	Counters Counters
	At       time.Time
}

// ParseStatus 从 SHOW GLOBAL STATUS 和少量变量取出本拍的瞬时值和累计计数。速率留给 ApplyRates。
func ParseStatus(fields, vars map[string]string) Sample {
	sample := Sample{
		ThreadsConnected:       atoi(fields["Threads_connected"]),
		ThreadsRunning:         atoi(fields["Threads_running"]),
		Questions:              atoi(fields["Questions"]),
		SlowQueries:            atoi(fields["Slow_queries"]),
		UptimeSec:              atoi(fields["Uptime"]),
		MaxConnections:         -1,
		BufferPoolHitRate:      -1,
		BufferPoolDirtyPercent: -1,
		QuestionsPerSec:        -1,
		SlowPerSec:             -1,
		SelectPerSec:           -1,
		InsertPerSec:           -1,
		UpdatePerSec:           -1,
		DeletePerSec:           -1,
		NetInKbps:              -1,
		NetOutKbps:             -1,
		ConnUsagePercent:       -1,
		AbortedPerSec:          -1,
		LockWaitsPerSec:        -1,
		TmpDiskPercent:         -1,
		ReplicaLagSec:          -1,
		ReplicaIO:              -1,
		ReplicaSQL:             -1,
		Role:                   "standalone",
	}
	sample.Counters = Counters{
		Questions:    sample.Questions,
		Slow:         sample.SlowQueries,
		Select:       atoi(fields["Com_select"]),
		Insert:       atoi(fields["Com_insert"]),
		Update:       atoi(fields["Com_update"]),
		Delete:       atoi(fields["Com_delete"]),
		BytesIn:      atoi(fields["Bytes_received"]),
		BytesOut:     atoi(fields["Bytes_sent"]),
		Aborted:      atoi(fields["Aborted_connects"]),
		LockWaits:    atoi(fields["Innodb_row_lock_waits"]),
		TmpTables:    atoi(fields["Created_tmp_tables"]),
		TmpDisk:      atoi(fields["Created_tmp_disk_tables"]),
		PoolReads:    atoi(fields["Innodb_buffer_pool_reads"]),
		PoolReadReqs: atoi(fields["Innodb_buffer_pool_read_requests"]),
		Uptime:       sample.UptimeSec,
	}
	total := atof(fields["Innodb_buffer_pool_pages_total"])
	dirty := atof(fields["Innodb_buffer_pool_pages_dirty"])
	if total > 0 {
		sample.BufferPoolDirtyPercent = dirty / total * 100
	}
	if n := atoi(vars["max_connections"]); n > 0 {
		sample.MaxConnections = n
		sample.ConnUsagePercent = float64(sample.ThreadsConnected) / float64(sample.MaxConnections) * 100
	}
	sample.PoolBytes = atoi(vars["innodb_buffer_pool_size"])
	return sample
}

// ApplyRates 用相邻两拍的增量计算每秒值和区间命中率。没有上一拍、实例重启或间隔异常时保持 -1。
func ApplyRates(sample *Sample, prev *Prior, now time.Time) {
	if sample == nil || prev == nil || sample.UptimeSec < prev.Counters.Uptime {
		return
	}
	sec := now.Sub(prev.At).Seconds()
	if sec < 1 || sec > 3600 {
		return
	}
	cur := sample.Counters
	old := prev.Counters
	sample.QuestionsPerSec = perSec(cur.Questions, old.Questions, sec)
	sample.SlowPerSec = perSec(cur.Slow, old.Slow, sec)
	sample.SelectPerSec = perSec(cur.Select, old.Select, sec)
	sample.InsertPerSec = perSec(cur.Insert, old.Insert, sec)
	sample.UpdatePerSec = perSec(cur.Update, old.Update, sec)
	sample.DeletePerSec = perSec(cur.Delete, old.Delete, sec)
	sample.AbortedPerSec = perSec(cur.Aborted, old.Aborted, sec)
	sample.LockWaitsPerSec = perSec(cur.LockWaits, old.LockWaits, sec)
	if in := perSec(cur.BytesIn, old.BytesIn, sec); in >= 0 {
		sample.NetInKbps = in / 1024
	}
	if out := perSec(cur.BytesOut, old.BytesOut, sec); out >= 0 {
		sample.NetOutKbps = out / 1024
	}
	dReq := cur.PoolReadReqs - old.PoolReadReqs
	dRead := cur.PoolReads - old.PoolReads
	if dReq > 0 && dRead >= 0 && dRead <= dReq {
		sample.BufferPoolHitRate = (1 - float64(dRead)/float64(dReq)) * 100
	}
	dTmp := cur.TmpTables - old.TmpTables
	dDisk := cur.TmpDisk - old.TmpDisk
	if dTmp > 0 && dDisk >= 0 && dDisk <= dTmp {
		sample.TmpDiskPercent = float64(dDisk) / float64(dTmp) * 100
	}
}

func perSec(cur, prev int64, sec float64) float64 {
	if cur < prev || sec <= 0 {
		return -1
	}
	return float64(cur-prev) / sec
}

// ThreadFlag 把复制线程状态收成数：Yes 为 1，No 为 0，其余为 -1。
func ThreadFlag(state string) float64 {
	switch state {
	case "Yes":
		return 1
	case "No":
		return 0
	default:
		return -1
	}
}
