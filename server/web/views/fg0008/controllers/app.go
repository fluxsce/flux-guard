package controllers

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0008/dao"
	"fluxguard/server/web/views/fg0008/fetch"
	"fluxguard/server/web/views/fg0008/inventory"
	"fluxguard/server/web/views/fg0008/release"
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

// QuerySites 返回应用代理看守的运行时套件（左侧：按 instance 导航）。
func (c *AppController) QuerySites(g *gin.Context) {
	p := middleware.Principal(g)
	rows, err := dao.ListSites(g.Request.Context(), c.db, p.TenantID, request.Param(g, "keyword"))
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载运行时失败")
		return
	}
	httpx.WriteOK(g, rows)
}

// QuerySiteFiles 返回一套运行时下的制品文件。
// mode=files：能拨则短拨列目录；隔离网拨不通则出站推清单。刷新失败时仍回退库内上次结果。
func (c *AppController) QuerySiteFiles(g *gin.Context) {
	p := middleware.Principal(g)
	agentID := strings.TrimSpace(request.Param(g, "agentId"))
	instanceName := strings.TrimSpace(request.Param(g, "instanceName"))
	if agentID == "" || instanceName == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少运行时")
		return
	}
	_, _ = inventory.Refresh(g.Request.Context(), c.db, p.TenantID, p.UserID, agentID, instanceName)
	rows, err := dao.ListSiteFiles(g.Request.Context(), c.db, p.TenantID, agentID, instanceName)
	if err != nil {
		writeMissing(g, err, "加载文件失败")
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
	canary := release.ParseCanary(g.PostForm("canaryCount"))
	audit.Note(g, "", "APP", appCode, appCode, "更新应用 "+kind+" "+version)
	out, err := release.Submit(g.Request.Context(), c.db, p.TenantID, p.UserID, release.Input{
		AppCode: appCode, Kind: kind, Version: version, File: header.Filename, Body: file, InstIDs: instIDs,
		CanaryCount: canary,
	})
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, out)
}

type releaseIDBody struct {
	ReleaseID string `json:"releaseId"`
}

// ContinueRelease 继续下发同一发布中尚未更新的实例。
func (c *AppController) ContinueRelease(g *gin.Context) {
	p := middleware.Principal(g)
	var body releaseIDBody
	if !request.Bind(g, &body) {
		return
	}
	releaseID := strings.TrimSpace(body.ReleaseID)
	if releaseID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少发布")
		return
	}
	audit.Note(g, "", "APP", releaseID, releaseID, "继续更新应用批次 "+releaseID)
	out, err := release.Continue(g.Request.Context(), c.db, p.TenantID, p.UserID, releaseID)
	if err != nil {
		writeMissing(g, err, "继续下发失败")
		return
	}
	httpx.WriteOK(g, out)
}

type rollbackBody struct {
	ReleaseID   string   `json:"releaseId"`
	InstIDs     []string `json:"instIds"`
	CanaryCount int      `json:"canaryCount"`
}

// Rollback 按一次已成功发布的备份回滚实例。
func (c *AppController) Rollback(g *gin.Context) {
	p := middleware.Principal(g)
	var body rollbackBody
	if !request.Bind(g, &body) {
		return
	}
	releaseID := strings.TrimSpace(body.ReleaseID)
	if releaseID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少发布")
		return
	}
	audit.Note(g, "", "APP", releaseID, releaseID, "回滚应用发布 "+releaseID)
	out, err := release.Rollback(g.Request.Context(), c.db, p.TenantID, p.UserID, releaseID, body.InstIDs, body.CanaryCount)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, out)
}

type fetchBody struct {
	InstID string `json:"instId"`
}

// RequestFileFetch 发起代下：能拨则中央短拨进暂存，拨不通则出站推送。
func (c *AppController) RequestFileFetch(g *gin.Context) {
	p := middleware.Principal(g)
	var body fetchBody
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "APP", body.InstID, body.InstID, "申请代下制品")
	out, err := fetch.Request(g.Request.Context(), c.db, p.TenantID, p.UserID, body.InstID)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, out)
}

// QueryFileFetch 查询代下任务状态。
func (c *AppController) QueryFileFetch(g *gin.Context) {
	p := middleware.Principal(g)
	jobID := strings.TrimSpace(request.Param(g, "jobId"))
	if jobID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少任务")
		return
	}
	out, err := fetch.Status(g.Request.Context(), c.db, p.TenantID, jobID)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, out)
}

// DownloadFileFetch 下载已就绪的暂存文件。
func (c *AppController) DownloadFileFetch(g *gin.Context) {
	p := middleware.Principal(g)
	jobID := strings.TrimSpace(request.Param(g, "jobId"))
	if jobID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少任务")
		return
	}
	file, name, err := fetch.Open(p.TenantID, jobID)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	if name == "" {
		name = filepath.Base(jobID)
	}
	g.Header("Content-Type", "application/octet-stream")
	g.Header("Content-Disposition", contentDisposition(name))
	g.Header("Content-Length", strconv.FormatInt(info.Size(), 10))
	g.Status(http.StatusOK)
	_, _ = io.Copy(g.Writer, file)
}

// contentDisposition 同时给 ASCII fallback 与 UTF-8 文件名，避免中文日志名乱码。
func contentDisposition(name string) string {
	safe := strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	if safe == "" {
		safe = "download"
	}
	escaped := url.PathEscape(name)
	return `attachment; filename="` + safe + `"; filename*=UTF-8''` + escaped
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

// QueryReleases 返回一个应用最近的发布记录。
func (c *AppController) QueryReleases(g *gin.Context) {
	p := middleware.Principal(g)
	appCode := strings.TrimSpace(request.Param(g, "appCode"))
	if appCode == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少应用")
		return
	}
	rows, err := dao.ListReleases(g.Request.Context(), c.db, p.TenantID, appCode, 20)
	if err != nil {
		writeMissing(g, err, "加载发布记录失败")
		return
	}
	httpx.WriteOK(g, rows)
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
