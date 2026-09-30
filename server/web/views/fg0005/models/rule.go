package models

// HostRule 是主机页看到的预警规则。指标和阈值存在通用规则的条件 JSON 里。
type HostRule struct {
	RuleID            string  `json:"ruleId"`
	RuleName          string  `json:"ruleName"`
	MetricCode        string  `json:"metricCode"`
	CompareOp         string  `json:"compareOp"`
	ThresholdValue    float64 `json:"thresholdValue"`
	DurationSec       int     `json:"durationSec"`
	LevelCode         string  `json:"levelCode"`
	AgentID           string  `json:"agentId"`
	ChannelID         string  `json:"channelId"`
	SilenceStart      string  `json:"silenceStart"`
	SilenceEnd        string  `json:"silenceEnd"`
	WaitMinutes       int     `json:"waitMinutes"`
	EscalateChannelID string  `json:"escalateChannelId"`
	StatusFlag        string  `json:"statusFlag"`
	ActiveFlag        string  `json:"activeFlag"`
	NoteText          string  `json:"noteText"`
	AddTime           string  `json:"addTime"`
	AddWho            string  `json:"addWho"`
	EditTime          string  `json:"editTime"`
	EditWho           string  `json:"editWho"`
	CurrentVersion    int     `json:"currentVersion"`
	OprSeqFlag        string  `json:"oprSeqFlag"`
}

// HostRuleInput 是主机预警规则的保存请求。
type HostRuleInput struct {
	RuleID            string  `json:"ruleId"`
	RuleName          string  `json:"ruleName"`
	MetricCode        string  `json:"metricCode"`
	CompareOp         string  `json:"compareOp"`
	ThresholdValue    float64 `json:"thresholdValue"`
	DurationSec       int     `json:"durationSec"`
	LevelCode         string  `json:"levelCode"`
	AgentID           string  `json:"agentId"`
	ChannelID         string  `json:"channelId"`
	SilenceStart      string  `json:"silenceStart"`
	SilenceEnd        string  `json:"silenceEnd"`
	WaitMinutes       int     `json:"waitMinutes"`
	EscalateChannelID string  `json:"escalateChannelId"`
	StatusFlag        string  `json:"statusFlag"`
	ActiveFlag        string  `json:"activeFlag"`
	NoteText          string  `json:"noteText"`
}
