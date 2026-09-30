package request

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
)

func TestBindJSONQueryURIAndHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/items/:id", func(c *gin.Context) {
		var body struct {
			ID    string `json:"id" uri:"id" binding:"required"`
			Name  string `json:"name" binding:"required"`
			Note  string `json:"note" form:"note"`
			Token string `json:"token" header:"X-Token" binding:"required"`
		}
		if !Bind(c, &body) {
			return
		}
		httpx.WriteOK(c, body)
	})

	missing := httptest.NewRequest(http.MethodPost, "/items/7?note=hi", strings.NewReader(`{}`))
	missing.Header.Set("Content-Type", "application/json")
	missing.Header.Set("X-Token", "tok")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, missing)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "缺少必填字段") {
		t.Fatalf("required %d %s", rec.Code, rec.Body.String())
	}

	ok := httptest.NewRequest(http.MethodPost, "/items/7?note=hi", strings.NewReader(`{"name":"host"}`))
	ok.Header.Set("Content-Type", "application/json")
	ok.Header.Set("X-Token", "tok")
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, ok)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"host"`) || !strings.Contains(rec.Body.String(), `"note":"hi"`) {
		t.Fatalf("bound %d %s", rec.Code, rec.Body.String())
	}
}

func TestBindAndParamShareJSONBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	type item struct {
		Name string `json:"name" binding:"required"`
	}
	engine.POST("/bind-first", func(c *gin.Context) {
		var body item
		if !Bind(c, &body) {
			return
		}
		httpx.WriteOK(c, map[string]string{"bound": body.Name, "param": Param(c, "name")})
	})
	engine.POST("/param-first", func(c *gin.Context) {
		name := Param(c, "name")
		var body item
		if !Bind(c, &body) {
			return
		}
		httpx.WriteOK(c, map[string]string{"bound": body.Name, "param": name})
	})

	for _, path := range []string{"/bind-first", "/param-first"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"name":"host"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"bound":"host"`) || !strings.Contains(rec.Body.String(), `"param":"host"`) {
			t.Fatalf("%s %d %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestStreamBindLeavesBodyUnread(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var reads int
	body := io.NopCloser(readerFunc(func(p []byte) (int, error) {
		reads++
		return 0, io.EOF
	}))
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/packages", body)
	c.Request.Header.Set("Content-Type", "application/octet-stream")
	c.Request.ContentLength = 200 << 20
	var dst struct {
		Name string `form:"name"`
	}
	if !Bind(c, &dst) {
		t.Fatalf("bind %d %s", rec.Code, rec.Body.String())
	}
	if reads != 0 {
		t.Fatalf("stream body reads %d", reads)
	}
}

func TestJSONOverCacheLimitRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var reads int64
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/items", io.NopCloser(readerFunc(func(p []byte) (int, error) {
		reads += int64(len(p))
		return 0, io.EOF
	})))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.ContentLength = maxJSONBytes + 1
	var dst struct {
		Name string `json:"name"`
	}
	if Bind(c, &dst) || rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "请求体过大") {
		t.Fatalf("too large %d %s", rec.Code, rec.Body.String())
	}
	if reads != 0 {
		t.Fatalf("oversized json reads %d", reads)
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

func TestBindForm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/form", func(c *gin.Context) {
		var body struct {
			Name string `json:"name" form:"name" binding:"required"`
		}
		if !Bind(c, &body) {
			return
		}
		httpx.WriteOK(c, body)
	})
	req := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader("name=agent"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"agent"`) {
		t.Fatalf("form %d %s", rec.Code, rec.Body.String())
	}
}
