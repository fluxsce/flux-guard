package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0022/dao"
	dbx "github.com/fluxsce/dbx/db"
)

// RoleController 角色查询、维护和授权。
type RoleController struct {
	roles *dao.RoleDAO
}

// NewRoleController 创建角色控制器。
func NewRoleController(db *dbx.DB) *RoleController {
	return &RoleController{roles: dao.NewRoleDAO(db)}
}

// QueryRoles 分页列出本租户角色。
func (c *RoleController) QueryRoles(g *gin.Context) {
	p := middleware.Principal(g)
	page := request.PageOf(g)
	rows, total, err := c.roles.PageRoles(p.TenantID, dao.RoleQuery{
		RoleName:    request.Param(g, "roleName"),
		RoleStatus:  request.Param(g, "roleStatus"),
		BuiltInFlag: request.Param(g, "builtInFlag"),
	}, page)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载角色失败")
		return
	}
	request.WritePage(g, rows, total, page)
}

// GetRole 按角色编码返回当前这一行。查看和修改都先取它，避免用列表里的旧数据。
func (c *RoleController) GetRole(g *gin.Context) {
	roleID := strings.TrimSpace(request.Param(g, "roleId"))
	if roleID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少角色")
		return
	}
	row, err := c.roles.GetRole(middleware.Principal(g).TenantID, roleID)
	if errors.Is(err, dao.ErrRoleMissing) {
		httpx.WriteErr(g, http.StatusNotFound, "角色不存在")
		return
	}
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载角色失败")
		return
	}
	httpx.WriteOK(g, row)
}

// QueryResources 列出资源，并标出指定角色已有的授权。
func (c *RoleController) QueryResources(g *gin.Context) {
	p := middleware.Principal(g)
	httpx.WriteOK(g, c.roles.Resources(p.TenantID, request.Param(g, "roleId")))
}

// CreateRole 新增自定义角色。成功时 data 为落库后的这一行，与列表项相同。
func (c *RoleController) CreateRole(g *gin.Context) {
	var body dao.RoleInput
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "ROLE", body.RoleID, body.RoleName, "新增角色 "+audit.Named(body.RoleName, body.RoleID))
	saved, err := c.roles.CreateRole(g.Request.Context(), body)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "ROLE", saved.RoleID, saved.RoleName, "新增角色 "+audit.Named(saved.RoleName, saved.RoleID))
	httpx.WriteOK(g, saved)
}

// UpdateRole 修改已有角色。成功时 data 为落库后的这一行，与列表项相同。
func (c *RoleController) UpdateRole(g *gin.Context) {
	var body dao.RoleInput
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "ROLE", body.RoleID, body.RoleName, "修改角色 "+audit.Named(body.RoleName, body.RoleID))
	saved, err := c.roles.UpdateRole(g.Request.Context(), body)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "ROLE", saved.RoleID, saved.RoleName, "修改角色 "+audit.Named(saved.RoleName, saved.RoleID))
	httpx.WriteOK(g, saved)
}

// DeleteRole 删除自定义角色。
func (c *RoleController) DeleteRole(g *gin.Context) {
	var body dao.RoleGrantInput
	if !request.Bind(g, &body) {
		return
	}
	id, name := c.roleLabel(g, body.RoleID)
	audit.Note(g, "", "ROLE", id, name, "删除角色 "+audit.Named(name, id))
	if err := c.roles.DeleteRole(g.Request.Context(), body.RoleID); err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "ROLE", id, name, "删除角色 "+audit.Named(name, id))
	httpx.WriteOK(g, map[string]any{"deleted": true})
}

// SaveRoleResources 按界面勾选替换角色授权。
func (c *RoleController) SaveRoleResources(g *gin.Context) {
	var body dao.RoleGrantInput
	if !request.Bind(g, &body) {
		return
	}
	id, name := c.roleLabel(g, body.RoleID)
	audit.Note(g, "", "ROLE", id, name, "为角色 "+audit.Named(name, id)+" 授权 "+strconv.Itoa(len(body.ResourceIDs))+" 项")
	if err := c.roles.SaveRoleResources(g.Request.Context(), body.RoleID, body.ResourceIDs); err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "ROLE", id, name, "为角色 "+audit.Named(name, id)+" 授权 "+strconv.Itoa(len(body.ResourceIDs))+" 项")
	httpx.WriteOK(g, map[string]any{"saved": true})
}

func (c *RoleController) roleLabel(g *gin.Context, roleID string) (id, name string) {
	id = strings.TrimSpace(roleID)
	name = id
	row, err := c.roles.GetRole(middleware.Principal(g).TenantID, id)
	if err == nil && strings.TrimSpace(row.RoleName) != "" {
		name = row.RoleName
	}
	return id, name
}
