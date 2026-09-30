package models

// Rule 是一条通用预警规则。条件 JSON 由功能模块解释，本包不拆字段。
type Rule struct {
	RuleID         string `json:"ruleId"`
	RuleName       string `json:"ruleName"`
	AlertType      string `json:"alertType"`
	LevelCode      string `json:"levelCode"`
	ChannelID      string `json:"channelId"`
	SilenceStart   string `json:"silenceStart"`
	SilenceEnd     string `json:"silenceEnd"`
	StatusFlag     string `json:"statusFlag"`
	ConditionText  string `json:"conditionText"`
	ActiveFlag     string `json:"activeFlag"`
	NoteText       string `json:"noteText"`
	AddTime        string `json:"addTime"`
	AddWho         string `json:"addWho"`
	EditTime       string `json:"editTime"`
	EditWho        string `json:"editWho"`
	CurrentVersion int    `json:"currentVersion"`
	OprSeqFlag     string `json:"oprSeqFlag"`
}

// RuleInput 是规则保存请求。编码留空时由服务端生成。ConditionText 必须是 JSON 对象。
type RuleInput struct {
	RuleID        string `json:"ruleId"`
	RuleName      string `json:"ruleName"`
	AlertType     string `json:"alertType"`
	LevelCode     string `json:"levelCode"`
	ChannelID     string `json:"channelId"`
	SilenceStart  string `json:"silenceStart"`
	SilenceEnd    string `json:"silenceEnd"`
	StatusFlag    string `json:"statusFlag"`
	ConditionText string `json:"conditionText"`
	ActiveFlag    string `json:"activeFlag"`
	NoteText      string `json:"noteText"`
}

// RuleQuery 是规则列表的筛选条件。ConditionMatch 是条件 JSON 里的一段原文。
type RuleQuery struct {
	RuleName       string
	AlertType      string
	LevelCode      string
	StatusFlag     string
	ConditionMatch string
}

// Channel 是渠道的公开视图。密钥不回传，SecretSet 表示库里是否已有密钥。
type Channel struct {
	ChannelID      string `json:"channelId"`
	ChannelName    string `json:"channelName"`
	ChannelKind    string `json:"channelKind"`
	Endpoint       string `json:"endpoint"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Sender         string `json:"sender"`
	Target         string `json:"target"`
	OptionText     string `json:"optionText"`
	ExtraText      string `json:"extraText"`
	TemplateID     string `json:"templateId"`
	TemplateName   string `json:"templateName"`
	SecretSet      string `json:"secretSet"`
	StatusFlag     string `json:"statusFlag"`
	ActiveFlag     string `json:"activeFlag"`
	NoteText       string `json:"noteText"`
	AddTime        string `json:"addTime"`
	AddWho         string `json:"addWho"`
	EditTime       string `json:"editTime"`
	EditWho        string `json:"editWho"`
	CurrentVersion int    `json:"currentVersion"`
	OprSeqFlag     string `json:"oprSeqFlag"`
}

// ChannelInput 是渠道保存请求。Secret 留空表示不修改已保存的密钥。
type ChannelInput struct {
	ChannelID   string `json:"channelId"`
	ChannelName string `json:"channelName"`
	ChannelKind string `json:"channelKind"`
	Endpoint    string `json:"endpoint"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Secret      string `json:"secret"`
	Sender      string `json:"sender"`
	Target      string `json:"target"`
	OptionText  string `json:"optionText"`
	ExtraText   string `json:"extraText"`
	TemplateID  string `json:"templateId"`
	StatusFlag  string `json:"statusFlag"`
	ActiveFlag  string `json:"activeFlag"`
	NoteText    string `json:"noteText"`
}

// Template 是一种渠道种类的标题和正文。场景用 {{scene}} 区分触发、恢复和升级。
type Template struct {
	TemplateID     string `json:"templateId"`
	TemplateName   string `json:"templateName"`
	ChannelKind    string `json:"channelKind"`
	SubjectText    string `json:"subjectText"`
	BodyText       string `json:"bodyText"`
	ContentType    string `json:"contentType"`
	BuiltInFlag    string `json:"builtInFlag"`
	StatusFlag     string `json:"statusFlag"`
	ActiveFlag     string `json:"activeFlag"`
	NoteText       string `json:"noteText"`
	AddTime        string `json:"addTime"`
	AddWho         string `json:"addWho"`
	EditTime       string `json:"editTime"`
	EditWho        string `json:"editWho"`
	CurrentVersion int    `json:"currentVersion"`
	OprSeqFlag     string `json:"oprSeqFlag"`
}

// TemplateInput 是模板保存请求。编码留空时由服务端生成。
type TemplateInput struct {
	TemplateID   string `json:"templateId"`
	TemplateName string `json:"templateName"`
	ChannelKind  string `json:"channelKind"`
	SubjectText  string `json:"subjectText"`
	BodyText     string `json:"bodyText"`
	ContentType  string `json:"contentType"`
	StatusFlag   string `json:"statusFlag"`
	ActiveFlag   string `json:"activeFlag"`
	NoteText     string `json:"noteText"`
}

// TemplateQuery 是模板列表的筛选条件。
type TemplateQuery struct {
	TemplateName string
	ChannelKind  string
	StatusFlag   string
}

// PreviewInput 用样例数据渲染标题和正文。正文留空时使用该种类的内置文案。
type PreviewInput struct {
	TemplateID  string `json:"templateId"`
	SubjectText string `json:"subjectText"`
	BodyText    string `json:"bodyText"`
	ChannelKind string `json:"channelKind"`
	SceneCode   string `json:"sceneCode"`
	ContentType string `json:"contentType"`
}

// Preview 是渲染结果。
type Preview struct {
	SubjectText string `json:"subjectText"`
	BodyText    string `json:"bodyText"`
	ContentType string `json:"contentType"`
}

// Contact 是通知接收人。
type Contact struct {
	ContactID    string `json:"contactId"`
	ContactName  string `json:"contactName"`
	EmailAddr    string `json:"emailAddr"`
	MobileNo     string `json:"mobileNo"`
	WecomUser    string `json:"wecomUser"`
	DingtalkUser string `json:"dingtalkUser"`
	QQAccount    string `json:"qqAccount"`
	FeishuUser   string `json:"feishuUser"`
	StatusFlag   string `json:"statusFlag"`
	ActiveFlag   string `json:"activeFlag"`
	NoteText     string `json:"noteText"`
	AddTime      string `json:"addTime"`
	AddWho       string `json:"addWho"`
	EditTime     string `json:"editTime"`
	EditWho      string `json:"editWho"`
}

// Log 是一条预警日志。发送结果写在同一行。
type Log struct {
	AlertLogID     string `json:"alertLogId"`
	AlertLevel     string `json:"alertLevel"`
	AlertType      string `json:"alertType"`
	SceneCode      string `json:"sceneCode"`
	AlertTitle     string `json:"alertTitle"`
	AlertContent   string `json:"alertContent"`
	AlertTimestamp string `json:"alertTimestamp"`
	ChannelID      string `json:"channelId"`
	SendStatus     string `json:"sendStatus"`
	SendTime       string `json:"sendTime"`
	SendResult     string `json:"sendResult"`
	SendError      string `json:"sendError"`
	FailCount      int    `json:"failCount"`
	AlertExtra     string `json:"alertExtra"`
	ActiveFlag     string `json:"activeFlag"`
	NoteText       string `json:"noteText"`
	AddTime        string `json:"addTime"`
	AddWho         string `json:"addWho"`
	EditTime       string `json:"editTime"`
	EditWho        string `json:"editWho"`
}

// LogQuery 是预警日志列表的筛选条件。
type LogQuery struct {
	AlertTitle string
	AlertLevel string
	AlertType  string
	SendStatus string
}

// IDInput 用于删除、确认和测试。各模块只填自己的编码。
type IDInput struct {
	RuleID     string `json:"ruleId"`
	ChannelID  string `json:"channelId"`
	TemplateID string `json:"templateId"`
}

// ChannelQuery 是渠道列表的筛选条件。
type ChannelQuery struct {
	ChannelName string
	ChannelKind string
	StatusFlag  string
}

// Notice 是功能模块决定发送时交给预警的一条日志。指标和阈值放在 Extra，预警不解释。
type Notice struct {
	Level     string
	Type      string
	Scene     string
	Title     string
	Content   string
	ChannelID string
	Extra     string
}
