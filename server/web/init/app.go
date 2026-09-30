// Package init 装配管理端：导入各入口的登记，创建引擎，并提供控制台页面。
package init

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	dbx "github.com/fluxsce/dbx/db"
)

// New 创建引擎并挂上已登记的入口和模块。各模块只用这条库连接。
func New(db *dbx.DB) http.Handler {
	router := routes.NewEngine()
	routes.Mount(router, db)
	return router
}

// Static 在已构建的控制台目录上提供页面。页面挂在 /fluxguard/，管理接口和上报接口仍交给 next。
// 页面和重定向在这里记访问日志。交给 next 的请求由 Gin 中间件记，避免同一条请求写两遍。
func Static(dir string, next http.Handler) http.Handler {
	root := filepath.Clean(dir)
	webRoot := routes.WebRoot
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleaned := path.Clean("/" + r.URL.Path)
		if cleaned != "/" && (isAPI(cleaned) || (r.Method != http.MethodGet && r.Method != http.MethodHead)) {
			next.ServeHTTP(w, r)
			return
		}
		visit := middleware.Begin(w, r)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		serveConsole(rec, r, root, webRoot, cleaned)
		middleware.Finish(r, rec.status, rec.size, visit)
	})
}

// serveConsole 只处理控制台页面：根路径重定向，其余按文件回退到 index.html。
func serveConsole(w http.ResponseWriter, r *http.Request, root, webRoot, cleaned string) {
	if cleaned == "/" {
		http.Redirect(w, r, webRoot+"/", http.StatusFound)
		return
	}
	if cleaned != webRoot && !strings.HasPrefix(cleaned, webRoot+"/") {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(cleaned, webRoot)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == "." {
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
		return
	}
	full := filepath.Join(root, filepath.FromSlash(rel))
	if full != root && !strings.HasPrefix(full, root+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
		return
	}
	http.ServeFile(w, r, full)
}

// statusRecorder 记下最终 HTTP 状态，供页面访问日志使用。
type statusRecorder struct {
	http.ResponseWriter
	status int
	size   int
	wrote  bool
}

func (w *statusRecorder) WriteHeader(code int) {
	if !w.wrote {
		w.status = code
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	if !w.wrote {
		w.status = http.StatusOK
		w.wrote = true
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func isAPI(path string) bool {
	return path == "/healthz" || strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, routes.APIRoot+"/")
}
