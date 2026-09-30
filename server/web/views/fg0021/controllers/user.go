package controllers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0021/dao"
	login "fluxguard/server/web/views/login/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

// UserController 用户查询、维护和角色授权。
type UserController struct {
	users *dao.UserDAO
}

// NewUserController 创建用户控制器。
func NewUserController(db *dbx.DB) *UserController {
	return &UserController{users: dao.NewUserDAO(db, dropSessions)}
}

func dropSessions(userID string) {
	if auth := login.Current(); auth != nil {
		auth.DropUser(userID)
	}
}

// QueryUsers 分页列出本租户用户。
func (c *UserController) QueryUsers(g *gin.Context) {
	p := middleware.Principal(g)
	page := request.PageOf(g)
	rows, total, err := c.users.PageUsers(p.TenantID, dao.UserQuery{
		UserName:   request.Param(g, "userName"),
		RealName:   request.Param(g, "realName"),
		StatusFlag: request.Param(g, "statusFlag"),
	}, page)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载用户失败")
		return
	}
	request.WritePage(g, rows, total, page)
}

// CreateUser 新增用户。成功时 data 为落库后的这一行，与列表项相同。
func (c *UserController) CreateUser(g *gin.Context) {
	var body dao.UserInput
	if !request.Bind(g, &body) {
		return
	}
	id, name := userLabel(body.RealName, body.UserName, body.UserID)
	intent := "新增用户 " + audit.Named(name, body.UserName)
	audit.Note(g, "", "USER", id, name, intent)
	saved, err := c.users.CreateUser(g.Request.Context(), body)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	id, name = userLabel(saved.RealName, saved.UserName, saved.UserID)
	audit.Note(g, "", "USER", id, name, "新增用户 "+audit.Named(name, saved.UserName))
	httpx.WriteOK(g, saved)
}

// UpdateUser 修改已有用户。成功时 data 为落库后的这一行，与列表项相同。
func (c *UserController) UpdateUser(g *gin.Context) {
	var body dao.UserInput
	if !request.Bind(g, &body) {
		return
	}
	id, name := userLabel(body.RealName, body.UserName, body.UserID)
	intent := "修改用户 " + audit.Named(name, body.UserName)
	audit.Note(g, "", "USER", id, name, intent)
	saved, err := c.users.UpdateUser(g.Request.Context(), body)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	id, name = userLabel(saved.RealName, saved.UserName, saved.UserID)
	detail := "修改用户 " + audit.Named(name, saved.UserName)
	if strings.TrimSpace(body.Password) != "" {
		detail += "，并重置了口令"
	}
	audit.Note(g, "", "USER", id, name, detail)
	httpx.WriteOK(g, saved)
}

// DeleteUser 删除用户。
func (c *UserController) DeleteUser(g *gin.Context) {
	var body dao.UserRoleInput
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "USER", body.UserID, body.UserID, "删除用户 "+body.UserID)
	if err := c.users.DeleteUser(g.Request.Context(), body.UserID); err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "USER", body.UserID, body.UserID, "删除用户 "+body.UserID)
	httpx.WriteOK(g, map[string]any{"deleted": true})
}

// QueryUserRoles 列出可分配角色，并标出该用户已有的角色。
func (c *UserController) QueryUserRoles(g *gin.Context) {
	p := middleware.Principal(g)
	roles, err := c.users.UserRoles(p.TenantID, request.Param(g, "userId"))
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, roles)
}

// SaveUserRoles 按界面勾选替换用户角色，可以同时保存多个。成功时 data 为带角色名的这一行。
func (c *UserController) SaveUserRoles(g *gin.Context) {
	var body dao.UserRoleInput
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "USER", body.UserID, body.UserID, "调整用户 "+body.UserID+" 的角色")
	saved, err := c.users.SaveUserRoles(g.Request.Context(), body.UserID, body.RoleIDs)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	id, name := userLabel(saved.RealName, saved.UserName, saved.UserID)
	who := audit.Named(name, saved.UserName)
	detail := "为用户 " + who + " 清空角色"
	if roles := strings.TrimSpace(saved.RoleNames); roles != "" {
		detail = "为用户 " + who + " 分配角色 " + roles
	}
	audit.Note(g, "", "USER", id, name, detail)
	httpx.WriteOK(g, saved)
}

func userLabel(realName, userName, userID string) (id, name string) {
	id = strings.TrimSpace(userID)
	if id == "" {
		id = strings.TrimSpace(userName)
	}
	name = strings.TrimSpace(realName)
	if name == "" {
		name = strings.TrimSpace(userName)
	}
	if name == "" {
		name = id
	}
	return id, name
}
