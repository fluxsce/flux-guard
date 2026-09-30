package controllers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0023/dao"
	"fluxguard/server/web/views/fg0023/models"
	dbx "github.com/fluxsce/dbx/db"
)

// ResourceController 资源目录查询。
type ResourceController struct {
	resources *dao.ResourceDAO
}

// NewResourceController 创建资源控制器。
func NewResourceController(db *dbx.DB) *ResourceController {
	return &ResourceController{resources: dao.NewResourceDAO(db)}
}

// QueryResources 按名称、编码、类型列出本租户当前启用的资源。
func (c *ResourceController) QueryResources(g *gin.Context) {
	rows, err := c.resources.List(g.Request.Context(), middleware.Principal(g).TenantID, dao.Filter{
		ResourceName: request.Param(g, "resourceName"),
		ResourceCode: request.Param(g, "resourceCode"),
		ResourceType: request.Param(g, "resourceType"),
	})
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载资源失败")
		return
	}
	httpx.WriteOK(g, models.Tree(rows))
}

// GetResource 按资源主键精确返回一行。
func (c *ResourceController) GetResource(g *gin.Context) {
	resourceID := strings.TrimSpace(request.Param(g, "resourceId"))
	if resourceID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少资源")
		return
	}
	row, err := c.resources.Get(g.Request.Context(), middleware.Principal(g).TenantID, resourceID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.WriteErr(g, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载资源失败")
		return
	}
	httpx.WriteOK(g, models.Detail(row))
}
