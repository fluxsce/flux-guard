package controllers

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/common/serverupdate"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware/audit"
)

// UpgradeServer 接收发行包并安排中央服务重启。
// 租户管理员或持有 fg0001:upgrade 的用户可调用。配置和数据库由升级过程保留。
func (c *OverviewController) UpgradeServer(g *gin.Context) {
	file, header, err := g.Request.FormFile("file")
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, "请选择发行包")
		return
	}
	defer file.Close()
	tmp, err := os.CreateTemp("", "fg-server-pack-*.tar.gz")
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "无法保存发行包")
		return
	}
	defer os.Remove(tmp.Name())
	written, copyErr := io.Copy(tmp, io.LimitReader(file, limits.ServerPackMaxBytes+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(copyErr, &tooLarge) {
			httpx.WriteErr(g, http.StatusBadRequest, "发行包超过 1GB")
			return
		}
		httpx.WriteErr(g, http.StatusBadRequest, "读取发行包失败")
		return
	}
	if closeErr != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "无法保存发行包")
		return
	}
	if written > limits.ServerPackMaxBytes {
		httpx.WriteErr(g, http.StatusBadRequest, "发行包超过 1GB")
		return
	}
	name := filepath.Base(header.Filename)
	sum := strings.TrimSpace(g.PostForm("sha256"))
	audit.Note(g, "", "SERVER", name, name, "升级中央服务 "+name)
	msg, err := serverupdate.Apply(tmp.Name(), name, sum)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, map[string]string{"message": msg})
}
