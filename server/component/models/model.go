// Package models 是组件目录、连接和曲线对外的数据结构。口令明文不在这些类型里。
package models

import "encoding/json"

// Metric 是目录里的一项指标，供页面给曲线列命名。
type Metric struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Unit string `json:"unit,omitempty"`
}

// Kind 是一种可直连的组件。
type Kind struct {
	KindCode    string   `json:"kindCode"`
	KindName    string   `json:"kindName"`
	DefaultPort int      `json:"defaultPort"`
	MetricTable string   `json:"metricTable"`
	SortOrder   int      `json:"sortOrder"`
	NoteText    string   `json:"noteText,omitempty"`
	Metrics     []Metric `json:"metrics"`
}

// Input 是保存连接的请求。Secret 留空表示修改时不更换口令。
type Input struct {
	ComponentID   string `json:"componentId"`
	KindCode      string `json:"kindCode"`
	ComponentName string `json:"componentName"`
	Address       string `json:"address"`
	UserName      string `json:"userName"`
	Secret        string `json:"secret"`
	DatabaseName  string `json:"databaseName"`
	OptionText    string `json:"optionText"`
	IntervalSec   int    `json:"intervalSec"`
	StatusFlag    string `json:"statusFlag"`
	NoteText      string `json:"noteText"`
}

// Conn 是一条连接的查询结果。SecretSet 表示是否已保存口令。
type Conn struct {
	ComponentID   string `json:"componentId"`
	KindCode      string `json:"kindCode"`
	KindName      string `json:"kindName"`
	ComponentName string `json:"componentName"`
	Address       string `json:"address"`
	UserName      string `json:"userName"`
	SecretSet     string `json:"secretSet"`
	DatabaseName  string `json:"databaseName"`
	OptionText    string `json:"optionText"`
	IntervalSec   int    `json:"intervalSec"`
	StatusFlag    string `json:"statusFlag"`
	UpFlag        string `json:"upFlag"`
	VersionText   string `json:"versionText"`
	RoleName      string `json:"roleName"`
	ErrorText     string `json:"errorText"`
	SummaryText   string `json:"summaryText"`
	ProbedTime    string `json:"probedTime"`
	NoteText      string `json:"noteText"`
	AddTime       string `json:"addTime"`
	EditTime      string `json:"editTime"`
}

// IDInput 指定一条连接。
type IDInput struct {
	ComponentID string `json:"componentId"`
}

// Trial 是弹窗里的一次试连，不落库。
type Trial struct {
	UpFlag      string `json:"upFlag"`
	VersionText string `json:"versionText"`
	RoleName    string `json:"roleName"`
	ErrorText   string `json:"errorText"`
	SummaryText string `json:"summaryText"`
}

// Status 是一次拨号的结果，写回连接行。
type Status struct {
	Up      bool
	Version string
	Role    string
	Error   string
	Summary string
}

// Point 是曲线上的一个点。Values 的键是目录里的指标编码。
type Point struct {
	ObservedTime string             `json:"observedTime"`
	UpFlag       string             `json:"upFlag"`
	VersionText  string             `json:"versionText"`
	RoleName     string             `json:"roleName"`
	ErrorText    string             `json:"errorText"`
	SummaryText  string             `json:"summaryText"`
	Values       map[string]float64 `json:"values"`
	Detail       json.RawMessage    `json:"detail,omitempty"`
}

// Latest 是某条连接最近一次采集。
type Latest struct {
	ComponentID string `json:"componentId"`
	KindCode    string `json:"kindCode"`
	Point       *Point `json:"point"`
}
