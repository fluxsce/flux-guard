// Package models 是应用管理页看到的应用和实例。
package models

// Site 是一套运行时（appside 配置里的 instance），左侧主列表按它导航。
// mode=files 时一套对应一个制品目录；Tomcat 一套对应 CATALINA_HOME（含 webapps 与 logs）。
type Site struct {
	AgentID        string `json:"agentId"`
	InstanceName   string `json:"instanceName"`
	HostName       string `json:"hostName"`
	IPAddress      string `json:"ipAddress,omitempty"`
	ModeCode       string `json:"modeCode"`
	RuntimeKind    string `json:"runtimeKind"`
	BasePath       string `json:"basePath,omitempty"`
	NoteText       string `json:"noteText,omitempty"`
	FileCount      int    `json:"fileCount"`
	OnlineFlag     string `json:"onlineFlag"`
	CommandAddress string `json:"commandAddress,omitempty"`
	TomcatVersion  string `json:"tomcatVersion,omitempty"`
	ReportedTime   string `json:"reportedTime,omitempty"`
}

// App 是按应用名汇总的一行。实例由应用侧代理上报，不在这里手工新增。
type App struct {
	AppCode       string `json:"appCode" db:"appCode"`
	AppName       string `json:"appName" db:"appName"`
	ArtifactKind  string `json:"artifactKind" db:"artifactKind"`
	InstanceCount int    `json:"instanceCount" db:"-"`
	VersionText   string `json:"versionText" db:"-"`
	MixedVersion  string `json:"mixedVersion" db:"-"`
	OnlineCount   int    `json:"onlineCount" db:"-"`
}

// Instance 是一个应用落在某台机器上的一套运行时；json 给 API，db 给 Select。
type Instance struct {
	InstID         string `json:"instId" db:"instId"`
	AgentID        string `json:"agentId" db:"agentId"`
	AppCode        string `json:"appCode" db:"appCode"`
	MachineID      string `json:"machineId" db:"machineId"`
	HostName       string `json:"hostName" db:"hostName"`
	IPAddress      string `json:"ipAddress,omitempty" db:"ipAddress"`
	InstanceName   string `json:"instanceName" db:"instanceName"`
	RuntimeKind    string `json:"runtimeKind" db:"runtimeKind"`
	ModeCode       string `json:"modeCode" db:"modeCode"`
	ArtifactKind   string `json:"artifactKind" db:"artifactKind"`
	ArtifactPath   string `json:"artifactPath,omitempty" db:"artifactPath"`
	VersionText    string `json:"versionText" db:"versionText"`
	TomcatVersion  string `json:"tomcatVersion" db:"tomcatVersion"`
	CommandAddress string `json:"commandAddress,omitempty" db:"commandAddress"`
	RunningFlag    string `json:"runningFlag" db:"runningFlag"`
	OnlineFlag     string `json:"onlineFlag" db:"-"`
	ReportedTime   string `json:"reportedTime" db:"reportedTime"`
	NoteText       string `json:"noteText,omitempty" db:"noteText,omitempty"`
}

// ReleaseStep 是一台实例上的可审计步骤（目标路径、备份路径），不是 CI 流水线。
type ReleaseStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	TargetPath string `json:"targetPath,omitempty"`
	BackupPath string `json:"backupPath,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// ReleaseItem 是一次发布里的一台实例；写入 FG_APP_RELEASE_ITEM 时 JOIN 投影列须为空并带 omitempty。
type ReleaseItem struct {
	CommandID      string        `json:"commandId" db:"commandId,pk"`
	TenantID       string        `json:"-" db:"tenantId,pk"`
	ReleaseID      string        `json:"-" db:"releaseId"`
	InstID         string        `json:"instId" db:"instId"`
	AgentID        string        `json:"agentId" db:"agentId"`
	AddTime        string        `json:"-" db:"addTime,noupdate,omitempty"`
	AddWho         string        `json:"-" db:"addWho,noupdate,omitempty"`
	EditTime       string        `json:"-" db:"editTime,omitempty"`
	EditWho        string        `json:"-" db:"editWho,omitempty"`
	OprSeqFlag     string        `json:"-" db:"oprSeqFlag,omitempty"`
	CurrentVersion int           `json:"-" db:"currentVersion,omitempty"`
	ActiveFlag     string        `json:"-" db:"activeFlag,omitempty"`
	StatusFlag     string        `json:"-" db:"statusFlag,omitempty"`
	Status         string        `json:"status" db:"-"`
	Message        string        `json:"message,omitempty" db:"resultText,omitempty"`
	NoteText       string        `json:"-" db:"noteText,omitempty"`
	Steps          []ReleaseStep `json:"steps,omitempty" db:"-"`
}

// Release 是一次更新或回滚的进度；同时供 API 与 dbx 单表读写。
type Release struct {
	ReleaseID       string        `json:"releaseId" db:"releaseId,pk"`
	TenantID        string        `json:"-" db:"tenantId,pk"`
	AppCode         string        `json:"appCode" db:"appCode"`
	ArtifactKind    string        `json:"artifactKind" db:"artifactKind"`
	VersionText     string        `json:"versionText" db:"versionText"`
	SHA256          string        `json:"-" db:"sha256"`
	FileName        string        `json:"fileName,omitempty" db:"fileName"`
	ExtProperty     string        `json:"-" db:"extProperty"`
	AddTime         string        `json:"addTime,omitempty" db:"addTime,noupdate"`
	AddWho          string        `json:"addWho,omitempty" db:"addWho,noupdate"`
	EditTime        string        `json:"-" db:"editTime"`
	EditWho         string        `json:"-" db:"editWho"`
	OprSeqFlag      string        `json:"-" db:"oprSeqFlag"`
	CurrentVersion  int           `json:"-" db:"currentVersion"`
	ActiveFlag      string        `json:"-" db:"activeFlag"`
	PendingCount    int           `json:"pendingCount,omitempty" db:"-"`
	BatchSize       int           `json:"batchSize,omitempty" db:"-"`
	Purpose         string        `json:"purpose,omitempty" db:"-"` // rollback 表示回滚发布
	SourceReleaseID string        `json:"sourceReleaseId,omitempty" db:"-"`
	Items           []ReleaseItem `json:"items,omitempty" db:"-"`
}
