package models

// AgentRelease 是一次代理自更新的汇总。
type AgentRelease struct {
	ReleaseID     string             `json:"releaseId"`
	AgentCode     string             `json:"agentCode"`
	VersionText   string             `json:"versionText"`
	FileName      string             `json:"fileName,omitempty"`
	GoOS          string             `json:"goos,omitempty"`
	GoArch        string             `json:"goarch,omitempty"`
	PendingCount  int                `json:"pendingCount"`
	BatchSize     int                `json:"batchSize,omitempty"`
	Items         []AgentReleaseItem `json:"items"`
}

// AgentReleaseItem 是一台代理上的命令状态与版本对照。
type AgentReleaseItem struct {
	CommandID      string `json:"commandId"`
	AgentID        string `json:"agentId"`
	AgentName      string `json:"agentName,omitempty"`
	Status         string `json:"status"`
	Message        string `json:"message,omitempty"`
	AgentVersion   string `json:"agentVersion,omitempty"`
	VersionMatched string `json:"versionMatched,omitempty"` // Y / N / 空（尚未对照）
}
