package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	"fluxguard/server/component/runtime"
	"fluxguard/server/component/service"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
)

// ComponentController 组件目录、连接和曲线。
type ComponentController struct {
	DB *dbx.DB
}

// NewComponentController 创建控制器。
func NewComponentController(db *dbx.DB) *ComponentController {
	return &ComponentController{DB: db}
}

func (c *ComponentController) ready(g *gin.Context) bool {
	if c == nil || c.DB == nil {
		httpx.WriteErr(g, http.StatusServiceUnavailable, "数据库未打开")
		return false
	}
	return true
}

// QueryCatalog 返回组件种类和指标名。种类和页签来自库表。
func (c *ComponentController) QueryCatalog(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	items, err := service.Catalog(g.Request.Context(), c.DB)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, items)
}

// QueryConns 分页列出连接。
func (c *ComponentController) QueryConns(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	kind := request.Param(g, "kindCode")
	if !c.allowKind(g, kind) {
		return
	}
	page := request.PageOf(g)
	rows, total, err := service.List(g.Request.Context(), c.DB, conn.Filter{
		KindCode:      kind,
		ComponentName: request.Param(g, "componentName"),
		StatusFlag:    request.Param(g, "statusFlag"),
		UpFlag:        request.Param(g, "upFlag"),
	}, page)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	request.WritePage(g, rows, total, page)
}

// CreateConn 新增连接。
func (c *ComponentController) CreateConn(g *gin.Context) {
	c.save(g, true)
}

// UpdateConn 修改连接。口令留空表示不改。
func (c *ComponentController) UpdateConn(g *gin.Context) {
	c.save(g, false)
}

func (c *ComponentController) save(g *gin.Context, creating bool) {
	if !c.ready(g) {
		return
	}
	var body models.Input
	if !request.Bind(g, &body) {
		return
	}
	if !c.allowKind(g, body.KindCode) {
		return
	}
	audit.Note(g, "", "COMPONENT", body.ComponentID, body.ComponentName, "")
	saved, err := service.Save(g.Request.Context(), c.DB, body, creating)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	verb := "修改组件连接 "
	if creating {
		verb = "新增组件连接 "
	}
	detail := verb + audit.Named(saved.ComponentName, saved.ComponentID)
	if saved.KindCode != "" {
		detail += "，种类 " + saved.KindCode
	}
	if strings.TrimSpace(body.Secret) != "" {
		detail += "，并更新了口令"
	}
	audit.Note(g, "", "COMPONENT", saved.ComponentID, saved.ComponentName, detail)
	httpx.WriteOK(g, saved)
}

// DeleteConn 停用连接。
func (c *ComponentController) DeleteConn(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	var body models.IDInput
	if !request.Bind(g, &body) {
		return
	}
	item, ok := c.owned(g, body.ComponentID)
	if !ok {
		return
	}
	audit.Note(g, "", "COMPONENT", body.ComponentID, item.ComponentName, "")
	if err := service.Delete(g.Request.Context(), c.DB, body.ComponentID); err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "COMPONENT", body.ComponentID, body.ComponentID, "删除组件连接 "+body.ComponentID)
	httpx.WriteOK(g, map[string]any{"deleted": true})
}

// TestConn 按弹窗填写试连，不落库。
func (c *ComponentController) TestConn(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	var body models.Input
	if !request.Bind(g, &body) {
		return
	}
	if !c.allowKind(g, body.KindCode) {
		return
	}
	audit.Note(g, "", "COMPONENT", body.ComponentID, body.ComponentName, "")
	trial, err := service.Test(g.Request.Context(), c.DB, body)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	detail := "测试组件连接 " + audit.Named(body.ComponentName, body.Address)
	if trial.UpFlag == "Y" {
		detail += "，已连通"
	} else {
		detail += "，未连通"
	}
	audit.Note(g, "", "COMPONENT", body.ComponentID, body.ComponentName, detail)
	httpx.WriteOK(g, trial)
}

// ProbeConn 立刻采集一次。
func (c *ComponentController) ProbeConn(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	var body models.IDInput
	if !request.Bind(g, &body) {
		return
	}
	if _, ok := c.owned(g, body.ComponentID); !ok {
		return
	}
	audit.Note(g, "", "COMPONENT", body.ComponentID, body.ComponentID, "")
	item, err := service.Probe(g.Request.Context(), c.DB, body.ComponentID)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	detail := "采集组件连接 " + audit.Named(item.ComponentName, item.ComponentID)
	if item.UpFlag != "" {
		detail += "，连通 " + item.UpFlag
	}
	audit.Note(g, "", "COMPONENT", item.ComponentID, item.ComponentName, detail)
	httpx.WriteOK(g, item)
}

// QueryLatest 返回最近一个点。
func (c *ComponentController) QueryLatest(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	if _, ok := c.owned(g, request.Param(g, "componentId")); !ok {
		return
	}
	item, err := service.Latest(g.Request.Context(), c.DB, request.Param(g, "componentId"))
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, item)
}

// ExecConn 对一条连接执行已登记的维护操作。ZooKeeper 是目录，MongoDB 是库、集合和索引，Redis 是键。
func (c *ComponentController) ExecConn(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	var body componentExec
	if !request.Bind(g, &body) {
		return
	}
	item, ok := c.owned(g, body.ComponentID)
	if !ok {
		return
	}
	command := strings.TrimSpace(body.Command)
	args := body.Args
	if args == nil {
		args = map[string]string{}
	}
	var note string
	switch item.KindCode {
	case catalog.ZK:
		if _, allowed := zkCommands[command]; !allowed {
			httpx.WriteErr(g, http.StatusBadRequest, "这种组件没有这项操作")
			return
		}
		if len(args["data"]) > 1<<20 {
			httpx.WriteErr(g, http.StatusBadRequest, "节点数据超过 1MB")
			return
		}
		if len(args["acl"]) > 64<<10 {
			httpx.WriteErr(g, http.StatusBadRequest, "权限数据过长")
			return
		}
		note = zkAudit(command, args)
	case catalog.Mongo:
		if _, allowed := mongoCommands[command]; !allowed {
			httpx.WriteErr(g, http.StatusBadRequest, "这种组件没有这项操作")
			return
		}
		for key, value := range args {
			limit := 512
			if key == "filter" {
				limit = 8192
			}
			if len(value) > limit {
				httpx.WriteErr(g, http.StatusBadRequest, "操作参数过长")
				return
			}
		}
		note = mongoAudit(command, args)
	case catalog.Kafka:
		if _, allowed := kafkaCommands[command]; !allowed {
			httpx.WriteErr(g, http.StatusBadRequest, "这种组件没有这项操作")
			return
		}
		for _, value := range args {
			if len(value) > 200 {
				httpx.WriteErr(g, http.StatusBadRequest, "操作参数过长")
				return
			}
		}
		note = kafkaAudit(command, args)
	case catalog.Redis:
		if _, allowed := redisCommands[command]; !allowed {
			httpx.WriteErr(g, http.StatusBadRequest, "这种组件没有这项操作")
			return
		}
		for key, value := range args {
			limit := 32
			switch key {
			case "name":
				limit = 1024
			case "match":
				limit = 256
			case "cursor":
				limit = 4096
			}
			if len(value) > limit {
				httpx.WriteErr(g, http.StatusBadRequest, "操作参数过长")
				return
			}
		}
		note = redisAudit(command, args)
	default:
		httpx.WriteErr(g, http.StatusBadRequest, "这种组件没有这项操作")
		return
	}
	principal := middleware.Principal(g)
	open, err := conn.LoadOpen(g.Request.Context(), c.DB, principal.TenantID, item.ComponentID)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "COMPONENT", item.ComponentID, item.ComponentName, note)
	output, err := runtime.Exec(g.Request.Context(), open, command, args)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	var parsed any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		httpx.WriteOK(g, map[string]string{"output": output})
		return
	}
	httpx.WriteOK(g, parsed)
}

func zkAudit(command string, args map[string]string) string {
	path := strings.TrimSpace(args["path"])
	if path == "" {
		path = "/"
	}
	switch command {
	case "create":
		return "新建 ZooKeeper 节点 " + path
	case "set":
		return "修改 ZooKeeper 节点 " + path
	case "delete":
		if args["recursive"] == "Y" {
			return "删除 ZooKeeper 子树 " + path
		}
		return "删除 ZooKeeper 节点 " + path
	case "setacl":
		return "修改 ZooKeeper 节点权限 " + path
	case "move":
		dest := strings.TrimSpace(args["dest"])
		if dest == "" {
			return "移动 ZooKeeper 节点 " + path
		}
		return "移动 ZooKeeper 节点 " + path + " 到 " + dest
	default:
		return "查看 ZooKeeper 节点 " + path
	}
}

// QuerySeries 返回曲线。
func (c *ComponentController) QuerySeries(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	componentID := request.Param(g, "componentId")
	if _, ok := c.owned(g, componentID); !ok {
		return
	}
	points, err := service.Series(g.Request.Context(), c.DB, componentID, request.Param(g, "limit"), request.Param(g, "from"), request.Param(g, "to"))
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, points)
}

func (c *ComponentController) owned(g *gin.Context, componentID string) (models.Conn, bool) {
	item, err := service.Lookup(g.Request.Context(), c.DB, componentID)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return models.Conn{}, false
	}
	if !c.allowKind(g, item.KindCode) {
		return models.Conn{}, false
	}
	return item, true
}

type componentExec struct {
	ComponentID string            `json:"componentId"`
	Command     string            `json:"command"`
	Args        map[string]string `json:"args"`
}

func mongoAudit(command string, args map[string]string) string {
	db := strings.TrimSpace(args["database"])
	coll := strings.TrimSpace(args["collection"])
	target := db
	if coll != "" {
		target = db + "." + coll
	}
	switch command {
	case "createDatabase":
		return "新建 MongoDB 库 " + db + "." + coll
	case "dropDatabase":
		return "删除 MongoDB 库 " + db
	case "createCollection":
		return "新建 MongoDB 集合 " + target
	case "dropCollection":
		return "删除 MongoDB 集合 " + target
	case "renameCollection":
		return "重命名 MongoDB 集合 " + target + " 为 " + strings.TrimSpace(args["name"])
	case "createIndex":
		return "新建 MongoDB 索引 " + target + " " + strings.TrimSpace(args["keys"])
	case "dropIndex":
		return "删除 MongoDB 索引 " + target + " " + strings.TrimSpace(args["name"])
	case "killOp":
		return "终止 MongoDB 操作 " + strings.TrimSpace(args["op"])
	case "find", "count", "explain":
		return "查询 MongoDB 集合 " + target
	case "profile":
		return "查看 MongoDB 慢日志 " + db
	default:
		if target == "" {
			return "查看 MongoDB 库"
		}
		return "查看 MongoDB " + target
	}
}

var zkCommands = map[string]struct{}{
	"children": {},
	"stat":     {},
	"get":      {},
	"create":   {},
	"set":      {},
	"delete":   {},
	"acl":      {},
	"setacl":   {},
	"move":     {},
}

var mongoCommands = map[string]struct{}{
	"databases":        {},
	"collections":      {},
	"stats":            {},
	"indexes":          {},
	"createDatabase":   {},
	"dropDatabase":     {},
	"createCollection": {},
	"dropCollection":   {},
	"renameCollection": {},
	"createIndex":      {},
	"dropIndex":        {},
	"killOp":           {},
	"find":             {},
	"count":            {},
	"explain":          {},
	"profile":          {},
}

func kafkaAudit(command string, args map[string]string) string {
	name := strings.TrimSpace(args["name"])
	switch command {
	case "create_topic":
		return "新建 Kafka 主题 " + name
	case "delete_topic":
		return "删除 Kafka 主题 " + name
	case "add_partitions":
		return "增加 Kafka 主题分区 " + name
	case "alter_config":
		return "修改 Kafka 主题配置 " + name + " " + strings.TrimSpace(args["key"])
	case "delete_group":
		return "删除 Kafka 消费组 " + name
	case "reset_offsets":
		return "重置 Kafka 消费组位点 " + name + " " + strings.TrimSpace(args["topic"])
	case "topic_config":
		return "查看 Kafka 主题配置 " + name
	case "groups":
		return "查看 Kafka 消费组"
	default:
		return "查看 Kafka 主题"
	}
}

func redisAudit(command string, args map[string]string) string {
	db := strings.TrimSpace(args["database"])
	if db == "" {
		db = "0"
	}
	name := strings.TrimSpace(args["name"])
	switch command {
	case "scan":
		return "查看 Redis 库 " + db + " 的键"
	case "key":
		return "查看 Redis 键 " + db + " " + name
	case "delete":
		return "删除 Redis 键 " + db + " " + name
	case "expire":
		if strings.TrimSpace(args["seconds"]) == "0" {
			return "取消 Redis 键过期 " + db + " " + name
		}
		return "设置 Redis 键过期 " + db + " " + name
	default:
		return "查看 Redis 库"
	}
}

var redisCommands = map[string]struct{}{
	"databases": {},
	"scan":      {},
	"key":       {},
	"delete":    {},
	"expire":    {},
}

var kafkaCommands = map[string]struct{}{
	"topics":         {},
	"topic_config":   {},
	"create_topic":   {},
	"delete_topic":   {},
	"add_partitions": {},
	"alter_config":   {},
	"groups":         {},
	"delete_group":   {},
	"reset_offsets":  {},
}

func (c *ComponentController) allowKind(g *gin.Context, kind string) bool {
	kind = strings.TrimSpace(kind)
	if kind == "" || !catalog.Known(kind) {
		httpx.WriteFail(g, http.StatusBadRequest, fmt.Errorf("请选择组件种类"))
		return false
	}
	principal := middleware.Principal(g)
	if principal.TenantAdmin {
		return true
	}
	access := middleware.Access()
	if access == nil || !access.AllowButton(principal.TenantID, principal.UserID, "fg0007:"+kind) {
		httpx.WriteErr(g, http.StatusForbidden, "没有权限")
		return false
	}
	return true
}
