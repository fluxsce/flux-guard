package models

// AgentRelease 是一次代理自更新的汇总；json 给 API，db 给 dbx Insert/Update/Get/Select。
type AgentRelease struct {
	ReleaseID      string             `json:"releaseId" db:"releaseId,pk"`
	TenantID       string             `json:"-" db:"tenantId,pk"`
	AgentCode      string             `json:"agentCode" db:"agentCode"`
	VersionText    string             `json:"versionText" db:"versionText"`
	SHA256         string             `json:"-" db:"sha256"`
	FileName       string             `json:"fileName,omitempty" db:"fileName"`
	GoOS           string             `json:"goos,omitempty" db:"goos"`
	GoArch         string             `json:"goarch,omitempty" db:"goarch"`
	ExtProperty    string             `json:"-" db:"extProperty"`
	AddTime        string             `json:"addTime,omitempty" db:"addTime,noupdate"`
	AddWho         string             `json:"addWho,omitempty" db:"addWho,noupdate"`
	EditTime       string             `json:"-" db:"editTime"`
	EditWho        string             `json:"-" db:"editWho"`
	OprSeqFlag     string             `json:"-" db:"oprSeqFlag"`
	CurrentVersion int                `json:"-" db:"currentVersion"`
	ActiveFlag     string             `json:"-" db:"activeFlag"`
	PendingCount   int                `json:"pendingCount" db:"-"`
	BatchSize      int                `json:"batchSize,omitempty" db:"-"`
	OpenFlag       string             `json:"openFlag,omitempty" db:"-"` // Y=仍有待继续或未结束命令
	Items          []AgentReleaseItem `json:"items" db:"-"`
}

// AgentReleaseItem 是一台代理上的命令状态与版本对照。
// 写入 FG_AGENT_RELEASE_ITEM 时，JOIN 投影列须为空并带 omitempty，避免插到不存在的列。
type AgentReleaseItem struct {
	CommandID      string `json:"commandId" db:"commandId,pk"`
	TenantID       string `json:"-" db:"tenantId,pk"`
	ReleaseID      string `json:"-" db:"releaseId"`
	AgentID        string `json:"agentId" db:"agentId"`
	AddTime        string `json:"-" db:"addTime,noupdate,omitempty"`
	AddWho         string `json:"-" db:"addWho,noupdate,omitempty"`
	EditTime       string `json:"-" db:"editTime,omitempty"`
	EditWho        string `json:"-" db:"editWho,omitempty"`
	OprSeqFlag     string `json:"-" db:"oprSeqFlag,omitempty"`
	CurrentVersion int    `json:"-" db:"currentVersion,omitempty"`
	ActiveFlag     string `json:"-" db:"activeFlag,omitempty"`
	AgentName      string `json:"agentName,omitempty" db:"agentName,omitempty"`
	StatusFlag     string `json:"-" db:"statusFlag,omitempty"`
	Status         string `json:"status" db:"-"`
	Message        string `json:"message,omitempty" db:"resultText,omitempty"`
	AgentVersion   string `json:"agentVersion,omitempty" db:"agentVersion,omitempty"`
	VersionMatched string `json:"versionMatched,omitempty" db:"-"` // Y / N / 空（尚未对照）
}

// AgentUpgradeStatus 是代理列表上展示的最近一次升级状态（每台代理最多一行）。
type AgentUpgradeStatus struct {
	AgentID        string `json:"agentId" db:"agentId"`
	ReleaseID      string `json:"releaseId" db:"-"`
	VersionText    string `json:"versionText" db:"-"`
	StatusFlag     string `json:"-" db:"statusFlag"`
	Status         string `json:"status" db:"-"` // pending / dialing / waiting / running / done / failed
	VersionMatched string `json:"versionMatched,omitempty" db:"-"`
	Message        string `json:"message,omitempty" db:"resultText"`
	AgentVersion   string `json:"-" db:"agentVersion"`
}
