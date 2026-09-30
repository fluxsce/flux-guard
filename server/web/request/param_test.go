package request

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
)

func TestParamReadsBodyWhenQueryMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/items/:id", func(c *gin.Context) {
		page := PageOf(c)
		httpx.WriteOK(c, map[string]any{
			"id":    Param(c, "id"),
			"name":  Param(c, "name"),
			"page":  page.Page,
			"size":  page.PageSize,
			"order": page.OrderBy,
		})
	})
	engine.POST("/form", func(c *gin.Context) {
		httpx.WriteOK(c, map[string]string{"name": Param(c, "name")})
	})

	body := httptest.NewRequest(http.MethodPost, "/items/7", strings.NewReader(`{"name":"host","page":2,"pageSize":15,"orderBy":"hostName"}`))
	body.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, body)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"host"`) || !strings.Contains(rec.Body.String(), `"id":"7"`) || !strings.Contains(rec.Body.String(), `"page":2`) || !strings.Contains(rec.Body.String(), `"order":"hostName"`) {
		t.Fatalf("json %d %s", rec.Code, rec.Body.String())
	}

	query := httptest.NewRequest(http.MethodPost, "/items/7?name=from-query", strings.NewReader(`{"name":"from-body"}`))
	query.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, query)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"from-query"`) {
		t.Fatalf("query wins %d %s", rec.Code, rec.Body.String())
	}

	form := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader("name=agent"))
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, form)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"agent"`) {
		t.Fatalf("form %d %s", rec.Code, rec.Body.String())
	}
}
