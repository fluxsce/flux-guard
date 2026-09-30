package models

import "fluxguard/server/setting"

// Settings 环境设置页一次拉取的全部分组。数据归档在 fg0025 任务调度。
type Settings struct {
	Editable    bool            `json:"editable"`
	WebTimeout  WebTimeout      `json:"webTimeout"`
	EnvVars     EnvVarsView     `json:"envVars"`
	IngestToken IngestTokenView `json:"ingestToken"`
}

// IngestTokenView 上报令牌回显。列表查询不带明文；查看接口和轮换应答带上 agentToken。
type IngestTokenView struct {
	Configured     bool   `json:"configured"`
	StatusFlag     string `json:"statusFlag"`
	Hint           string `json:"hint"`
	CurrentVersion int    `json:"currentVersion"`
	AgentToken     string `json:"agentToken,omitempty"`
}

// WebTimeout 访问策略回显。currentVersion 为 0 表示尚未落库。
type WebTimeout struct {
	RequestTimeoutSeconds int  `json:"requestTimeoutSeconds"`
	SessionExpireHours    int  `json:"sessionExpireHours"`
	DefaultPageSize       int  `json:"defaultPageSize"`
	MaxPageSize           int  `json:"maxPageSize"`
	CurrentVersion        int  `json:"currentVersion"`
	Editable              bool `json:"editable"`
}

// EnvVarsView 全局环境变量回显。
type EnvVarsView struct {
	Items          []setting.EnvVarView `json:"items"`
	CurrentVersion int                  `json:"currentVersion"`
}

// SaveRequest 按分组保存。groupCode 为空时按访问策略处理。
// 上报令牌的 action 为 rotate、enable、disable。
type SaveRequest struct {
	GroupCode             string                `json:"groupCode"`
	Action                string                `json:"action"`
	CurrentVersion        int                   `json:"currentVersion"`
	RequestTimeoutSeconds int                   `json:"requestTimeoutSeconds"`
	SessionExpireHours    int                   `json:"sessionExpireHours"`
	DefaultPageSize       int                   `json:"defaultPageSize"`
	MaxPageSize           int                   `json:"maxPageSize"`
	Items                 []setting.EnvVarInput `json:"items"`
}
