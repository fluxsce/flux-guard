// Package models 是应用管理页看到的应用和实例。
package models

// App 是按应用名汇总的一行。实例由应用侧代理上报，不在这里手工新增。
type App struct {
	AppCode       string `json:"appCode"`
	AppName       string `json:"appName"`
	ArtifactKind  string `json:"artifactKind"`
	InstanceCount int    `json:"instanceCount"`
	VersionText   string `json:"versionText"`
	MixedVersion  string `json:"mixedVersion"`
	OnlineCount   int    `json:"onlineCount"`
}

// Instance 是一个应用落在某台机器上的一套运行时。
type Instance struct {
	InstID        string `json:"instId"`
	AgentID       string `json:"agentId"`
	AppCode       string `json:"appCode"`
	MachineID     string `json:"machineId"`
	HostName      string `json:"hostName"`
	InstanceName  string `json:"instanceName"`
	RuntimeKind   string `json:"runtimeKind"`
	ModeCode      string `json:"modeCode"`
	ArtifactKind  string `json:"artifactKind"`
	VersionText   string `json:"versionText"`
	TomcatVersion string `json:"tomcatVersion"`
	RunningFlag   string `json:"runningFlag"`
	OnlineFlag    string `json:"onlineFlag"`
	ReportedTime  string `json:"reportedTime"`
}

// ReleaseItem 是一次发布里的一台实例。
type ReleaseItem struct {
	CommandID string `json:"commandId"`
	InstID    string `json:"instId"`
	AgentID   string `json:"agentId"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

// Release 是一次更新的进度。
type Release struct {
	ReleaseID    string        `json:"releaseId"`
	AppCode      string        `json:"appCode"`
	ArtifactKind string        `json:"artifactKind"`
	VersionText  string        `json:"versionText"`
	FileName     string        `json:"fileName,omitempty"`
	AddTime      string        `json:"addTime,omitempty"`
	AddWho       string        `json:"addWho,omitempty"`
	Items        []ReleaseItem `json:"items,omitempty"`
}
