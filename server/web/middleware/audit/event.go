// Package audit 把管理端写操作和登录结果写入 FG_AUTH_AUDIT_LOG。
// 路由在引擎上挂 Handler，请求结束时落库。查询和只读按钮不记。
// 业务在绑定之后调用 Note，补上对象和一句操作说明。身份、路径、结果与失败原因由 Handler 补齐。
// detailText 只写业务说明；失败原因写入 failReason，不拼进 detailText。不读取请求体，避免把口令、令牌和验证码写进审计。
package audit

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// ResultOK 表示这次操作成功。
	ResultOK = "Y"
	// ResultFail 表示这次操作失败。
	ResultFail = "N"

	// ActionCreate 新增。
	ActionCreate = "CREATE"
	// ActionUpdate 修改。
	ActionUpdate = "UPDATE"
	// ActionDelete 删除。
	ActionDelete = "DELETE"
	// ActionGrant 授权。
	ActionGrant = "GRANT"
	// ActionCommand 向代理下发命令。
	ActionCommand = "COMMAND"
	// ActionLogin 登录成功。
	ActionLogin = "LOGIN"
	// ActionLoginFail 登录失败。
	ActionLoginFail = "LOGIN_FAIL"
	// ActionLogout 登出。
	ActionLogout = "LOGOUT"

	maxDetail     = 2000
	maxFailReason = 500

	eventKey    = "fluxguard.audit.event"
	resourceKey = "fluxguard.audit.resource"
	flushedKey  = "fluxguard.audit.flushed"

	// apiRoot 与 routes.APIRoot 相同。本包不引用 routes，避免和中间件形成循环依赖。
	apiRoot = "/fluxguard/api"
)

// Event 是一条控制面审计。调用方填写动作、目标和操作说明，其余字段可留空。
type Event struct {
	UserID        string
	TenantID      string
	UserName      string
	Action        string
	ModuleCode    string
	TargetType    string
	TargetID      string
	TargetName    string
	ResourceCode  string
	RequestPath   string
	RequestMethod string
	ClientIP      string
	Result        string
	Detail        string
	FailReason    string
}

// SetEvent 把这条审计放进当前请求。同一请求只保留最后一次。
// 登录还没有会话时也要调用，否则失败登录对不上操作者。
func SetEvent(c *gin.Context, event *Event) {
	if c == nil || event == nil {
		return
	}
	c.Set(eventKey, event)
}

// Note 记下这次写操作改了谁、改了什么。action 留空时仍按路径判断。
// detail 写一句操作说明（做什么），不要放口令、令牌、密钥、环境变量的值或终端按键。
// 失败原因由 Handler 写入 FailReason，不要塞进 detail。
func Note(c *gin.Context, action, targetType, targetID, targetName, detail string) {
	if c == nil {
		return
	}
	SetEvent(c, &Event{
		Action:     strings.TrimSpace(action),
		TargetType: strings.TrimSpace(targetType),
		TargetID:   strings.TrimSpace(targetID),
		TargetName: strings.TrimSpace(targetName),
		Detail:     strings.TrimSpace(detail),
	})
}

// Named 把名称和编码拼成审计里看到的对象。两者相同或只剩一个时不重复。
func Named(name, id string) string {
	name = strings.TrimSpace(name)
	id = strings.TrimSpace(id)
	if name != "" && id != "" && name != id {
		return name + "（" + id + "）"
	}
	if name != "" {
		return name
	}
	return id
}

// MarkResource 记下路由上声明的按钮码，供落库时填写 resourceCode 和动作。
func MarkResource(c *gin.Context, resource string) {
	if c == nil || resource == "" {
		return
	}
	c.Set(resourceKey, resource)
}

func eventFrom(c *gin.Context) *Event {
	if c == nil {
		return nil
	}
	raw, ok := c.Get(eventKey)
	if !ok || raw == nil {
		return nil
	}
	event, _ := raw.(*Event)
	return event
}

func resourceFrom(c *gin.Context) string {
	if c == nil {
		return ""
	}
	raw, ok := c.Get(resourceKey)
	if !ok || raw == nil {
		return ""
	}
	resource, _ := raw.(string)
	return resource
}

func alreadyFlushed(c *gin.Context) bool {
	if c == nil {
		return true
	}
	raw, ok := c.Get(flushedKey)
	if !ok {
		return false
	}
	flag, _ := raw.(bool)
	return flag
}

// ActionOf 按路径末段和按钮码判断要记的动作。只读返回空串。
func ActionOf(path, resource string) string {
	if readOnlyResource(resource) {
		return ""
	}
	base := lastSegment(path)
	switch {
	case base == "captcha":
		return ""
	case base == "login":
		return ActionLogin
	case base == "logout":
		return ActionLogout
	case strings.HasPrefix(base, "query"), strings.HasPrefix(base, "preview"):
		return ""
	case base == "saveUserRoles", base == "saveRoleResources":
		return ActionGrant
	case strings.HasPrefix(base, "delete"):
		return ActionDelete
	case strings.HasPrefix(base, "create"), strings.HasPrefix(base, "register"):
		return ActionCreate
	case strings.HasPrefix(base, "dispatch"), base == "openShell", base == "closeShell":
		return ActionCommand
	case strings.HasPrefix(base, "update"), strings.HasPrefix(base, "save"),
		strings.HasPrefix(base, "ack"), strings.HasPrefix(base, "resolve"),
		strings.HasPrefix(base, "test"), base == "profile":
		return ActionUpdate
	default:
		return ""
	}
}

func readOnlyResource(resource string) bool {
	if resource == "" {
		return false
	}
	for _, suffix := range []string{":view", ":search", ":query"} {
		if strings.HasSuffix(resource, suffix) {
			return true
		}
	}
	return false
}

func lastSegment(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimRight(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func moduleOf(path, resource string) string {
	if i := strings.Index(resource, ":"); i > 0 {
		return resource[:i]
	}
	prefix := apiRoot + "/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	seg := rest
	if i := strings.Index(rest, "/"); i >= 0 {
		seg = rest[:i]
	}
	if strings.HasPrefix(seg, "fg") {
		return seg
	}
	if seg == "auth" {
		return "login"
	}
	return ""
}

func targetTypeOf(path string) string {
	base := lastSegment(path)
	switch {
	case strings.Contains(base, "User"), base == "profile", base == "login", base == "logout":
		return "USER"
	case strings.Contains(base, "Role"), base == "saveRoleResources":
		return "ROLE"
	case strings.Contains(base, "Agent"), strings.Contains(base, "Command"):
		return "AGENT"
	case strings.Contains(base, "Setting"):
		return "SETTING"
	case strings.Contains(base, "Rule"):
		return "RULE"
	case strings.Contains(base, "Channel"):
		return "CHANNEL"
	case strings.Contains(base, "Template"):
		return "TEMPLATE"
	case strings.Contains(base, "Event"):
		return "EVENT"
	default:
		return ""
	}
}

func managementPath(path string) bool {
	return path == apiRoot || strings.HasPrefix(path, apiRoot+"/")
}
