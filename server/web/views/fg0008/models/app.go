// Package models 是应用管理页看到的应用和实例。
package models

// Site 是一套运行时（appside 配置里的 instance），左侧主列表按它导航。
// mode=files 时一套对应一个制品目录；Tomcat 一套对应一台 webapps。
type Site struct {
	AgentID        string `json:"agentId"`
	InstanceName   string `json:"instanceName"`
	HostName       string `json:"hostName"`
	IPAddress      string `json:"ipAddress,omitempty"`
	ModeCode       string `json:"modeCode"`
	RuntimeKind    string `json:"runtimeKind"`
	BasePath       string `json:"basePath,omitempty"`
	FileCount      int    `json:"fileCount"`
	OnlineFlag     string `json:"onlineFlag"`
	CommandAddress string `json:"commandAddress,omitempty"`
	TomcatVersion  string `json:"tomcatVersion,omitempty"`
	ReportedTime   string `json:"reportedTime,omitempty"`
}

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
	InstID         string `json:"instId"`
	AgentID        string `json:"agentId"`
	AppCode        string `json:"appCode"`
	MachineID      string `json:"machineId"`
	HostName       string `json:"hostName"`
	IPAddress      string `json:"ipAddress,omitempty"`
	InstanceName   string `json:"instanceName"`
	RuntimeKind    string `json:"runtimeKind"`
	ModeCode       string `json:"modeCode"`
	ArtifactKind   string `json:"artifactKind"`
	ArtifactPath   string `json:"artifactPath,omitempty"`
	VersionText    string `json:"versionText"`
	TomcatVersion  string `json:"tomcatVersion"`
	CommandAddress string `json:"commandAddress,omitempty"`
	RunningFlag    string `json:"runningFlag"`
	OnlineFlag     string `json:"onlineFlag"`
	ReportedTime   string `json:"reportedTime"`
}

// ReleaseStep 是一台实例上的可审计步骤（目标路径、备份路径），不是 CI 流水线。
type ReleaseStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	TargetPath string `json:"targetPath,omitempty"`
	BackupPath string `json:"backupPath,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// ReleaseItem 是一次发布里的一台实例。
type ReleaseItem struct {
	CommandID string        `json:"commandId"`
	InstID    string        `json:"instId"`
	AgentID   string        `json:"agentId"`
	Status    string        `json:"status"`
	Message   string        `json:"message,omitempty"`
	Steps     []ReleaseStep `json:"steps,omitempty"`
}

// Release 是一次更新或回滚的进度。
type Release struct {
	ReleaseID       string        `json:"releaseId"`
	AppCode         string        `json:"appCode"`
	ArtifactKind    string        `json:"artifactKind"`
	VersionText     string        `json:"versionText"`
	FileName        string        `json:"fileName,omitempty"`
	AddTime         string        `json:"addTime,omitempty"`
	AddWho          string        `json:"addWho,omitempty"`
	PendingCount    int           `json:"pendingCount,omitempty"`
	BatchSize       int           `json:"batchSize,omitempty"`
	Purpose         string        `json:"purpose,omitempty"` // rollback 表示回滚发布
	SourceReleaseID string        `json:"sourceReleaseId,omitempty"`
	Items           []ReleaseItem `json:"items,omitempty"`
}
