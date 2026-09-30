package models

import "time"

// Event 一条已接收的事件。
type Event struct {
	EventID     string            `json:"eventId"`
	TenantID    string            `json:"tenantId"`
	AgentID     string            `json:"agentId"`
	AgentCode   string            `json:"agentCode"`
	EventName   string            `json:"eventName"`
	Severity    string            `json:"severity"`
	MessageText string            `json:"messageText,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	ObservedAt  time.Time         `json:"observedAt"`
	BatchID     string            `json:"batchId"`
}

// Stats 是库里事件条数和最近一次接收时间。
type Stats struct {
	EventCount       int
	LastReceivedTime *time.Time
}
