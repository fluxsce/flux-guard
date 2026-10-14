package command

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"fluxguard/protocol"
)

// FileOpen 打开本机允许下载的文件。path 由调用方校验落在看守目录内。
type FileOpen func(path string) (name string, size int64, body io.ReadCloser, err error)

// FileList 按实例名实时列出看守目录内的制品（files 模式用）。
type FileList func(instance string) ([]protocol.AppArtifact, error)

// FileListHandler 提供 GET 列目录。?instance= 必填。
func FileListHandler(token string, list FileList) http.Handler {
	token = strings.TrimSpace(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if list == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if !allowKey(r.Header.Get("Authorization"), token) {
			http.Error(w, "代理令牌无效", http.StatusUnauthorized)
			return
		}
		instance := strings.TrimSpace(r.URL.Query().Get("instance"))
		if instance == "" {
			http.Error(w, "缺少实例", http.StatusBadRequest)
			return
		}
		items, err := list(instance)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if items == nil {
			items = []protocol.AppArtifact{}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(items)
	})
}

// FileHandler 提供 GET 文件下载。缺令牌或路径非法时拒绝。
func FileHandler(token string, open FileOpen) http.Handler {
	token = strings.TrimSpace(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if open == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if !allowKey(r.Header.Get("Authorization"), token) {
			http.Error(w, "代理令牌无效", http.StatusUnauthorized)
			return
		}
		path := strings.TrimSpace(r.URL.Query().Get("path"))
		if path == "" || strings.Contains(path, "..") {
			http.Error(w, "路径无效", http.StatusBadRequest)
			return
		}
		name, size, body, err := open(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer body.Close()
		if name == "" {
			name = filepath.Base(path)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeFileName(name)+`"`)
		if size >= 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		}
		_, _ = io.Copy(w, body)
	})
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, `"`, "")
	if name == "" || name == "." || name == ".." {
		return "download.bin"
	}
	return name
}
