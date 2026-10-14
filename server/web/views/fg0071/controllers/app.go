package controllers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0071/dao"
	"fluxguard/server/web/views/fg0071/release"
	dbx "github.com/fluxsce/dbx/db"
)

// AppController 提供应用列表、改名、忽略和更新。
type AppController struct {
	db *dbx.DB
}

// NewAppController 创建应用管理控制器。
func NewAppController(db *dbx.DB) *AppController {
	return &AppController{db: db}
}

// QueryApps 返回上报生成、且未被忽略的应用。
func (c *AppController) QueryApps(g *gin.Context) {
	p := middleware.Principal(g)
	rows, err := dao.ListApps(g.Request.Context(), c.db, p.TenantID, request.Param(g, "appName"))
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载应用失败")
		return
	}
	httpx.WriteOK(g, rows)
}

// QueryInstances 返回一个应用下的实例。
func (c *AppController) QueryInstances(g *gin.Context) {
	p := middleware.Principal(g)
	appCode := strings.TrimSpace(request.Param(g, "appCode"))
	if appCode == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少应用")
		return
	}
	rows, err := dao.ListInstances(g.Request.Context(), c.db, p.TenantID, appCode)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载实例失败")
		return
	}
	httpx.WriteOK(g, rows)
}

type nameBody struct {
	AppCode string `json:"appCode"`
	AppName string `json:"appName"`
}

// Rename 只改显示名。
func (c *AppController) Rename(g *gin.Context) {
	p := middleware.Principal(g)
	var body nameBody
	if !request.Bind(g, &body) {
		return
	}
	appCode := strings.TrimSpace(body.AppCode)
	appName := strings.TrimSpace(body.AppName)
	if appCode == "" || appName == "" || len(appName) > 100 {
		httpx.WriteErr(g, http.StatusBadRequest, "名称无效")
		return
	}
	audit.Note(g, "", "APP", appCode, appName, "修改应用显示名")
	if err := dao.Rename(g.Request.Context(), c.db, p.TenantID, appCode, appName, p.UserID); err != nil {
		writeMissing(g, err, "修改失败")
		return
	}
	httpx.WriteOK(g, map[string]string{"appCode": appCode, "appName": appName})
}

type codeBody struct {
	AppCode string `json:"appCode"`
}

// Ignore 从列表隐藏这个应用。
func (c *AppController) Ignore(g *gin.Context) {
	p := middleware.Principal(g)
	var body codeBody
	if !request.Bind(g, &body) {
		return
	}
	appCode := strings.TrimSpace(body.AppCode)
	if appCode == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少应用")
		return
	}
	audit.Note(g, "", "APP", appCode, appCode, "忽略应用")
	if err := dao.Ignore(g.Request.Context(), c.db, p.TenantID, appCode, p.UserID); err != nil {
		writeMissing(g, err, "忽略失败")
		return
	}
	httpx.WriteOK(g, map[string]string{"appCode": appCode})
}

// Release 上传制品，并向选中的实例下发更新。
func (c *AppController) Release(g *gin.Context) {
	p := middleware.Principal(g)
	file, header, err := g.Request.FormFile("file")
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, "请选择文件")
		return
	}
	defer file.Close()
	if header.Size > 512<<20 {
		httpx.WriteErr(g, http.StatusBadRequest, "文件超过 512MB")
		return
	}
	appCode := strings.TrimSpace(g.PostForm("appCode"))
	kind := strings.TrimSpace(g.PostForm("kind"))
	version := strings.TrimSpace(g.PostForm("version"))
	instIDs := splitIDs(g.PostForm("instIds"))
	audit.Note(g, "", "APP", appCode, appCode, "更新应用 "+kind+" "+version)
	out, err := release.Submit(g.Request.Context(), c.db, p.TenantID, p.UserID, release.Input{
		AppCode: appCode, Kind: kind, Version: version, File: header.Filename, Body: file, InstIDs: instIDs,
	})
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, out)
}

// QueryRelease 返回一次更新里各实例的命令状态。
func (c *AppController) QueryRelease(g *gin.Context) {
	p := middleware.Principal(g)
	releaseID := strings.TrimSpace(request.Param(g, "releaseId"))
	if releaseID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少发布")
		return
	}
	out, err := release.Progress(g.Request.Context(), c.db, p.TenantID, releaseID)
	if err != nil {
		writeMissing(g, err, "加载发布失败")
		return
	}
	httpx.WriteOK(g, out)
}

func splitIDs(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func writeMissing(g *gin.Context, err error, fallback string) {
	if errors.Is(err, dao.ErrMissing) {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteErr(g, http.StatusInternalServerError, fallback)
}
