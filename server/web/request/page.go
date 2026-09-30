package request

import (
	"reflect"
	"strconv"
	"strings"

	"fluxguard/server/web/httpx"
	dbx "github.com/fluxsce/dbx/db"
	"github.com/gin-gonic/gin"
)

// PageData 是列表接口的统一分页体。
type PageData struct {
	Items    any   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// PageOf 读取页码和每页条数，并按租户设置收束 pageSize。
// 查询串、路径和正文都能取到，避免参数只写在 JSON 里时页码丢失。
// orderBy 原样带回，是否允许由各查询的列白名单决定。
func PageOf(c *gin.Context) dbx.Page {
	page, _ := strconv.Atoi(Param(c, "page"))
	if page < 1 {
		page = 1
	}
	sizeRaw := Param(c, "pageSize")
	size, sizeErr := strconv.Atoi(sizeRaw)
	tenantID := ""
	if sess, ok := Current(c.Request.Context()); ok {
		tenantID = sess.TenantID
	}
	size = PageSize(tenantID, size, sizeRaw != "" && sizeErr == nil)
	descRaw := Param(c, "desc")
	desc := descRaw == "1" || strings.EqualFold(descRaw, "true")
	return dbx.Page{Page: page, PageSize: size, OrderBy: strings.TrimSpace(Param(c, "orderBy")), Desc: desc}
}

// WritePage 写出一页列表。items 为空时写成空数组。
func WritePage(c *gin.Context, items any, total int64, page dbx.Page) {
	if items == nil || sliceNil(items) {
		items = []any{}
	}
	page = page.Normalize()
	httpx.WriteOK(c, PageData{
		Items:    items,
		Total:    total,
		Page:     page.Page,
		PageSize: page.PageSize,
	})
}

func sliceNil(items any) bool {
	value := reflect.ValueOf(items)
	return value.Kind() == reflect.Slice && value.IsNil()
}
