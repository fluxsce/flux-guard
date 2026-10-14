package appside

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// OpenWatchedFile 打开看守目录内的文件供中央代下。路径必须落在某套 runtime 的 base / jar / static / backup 下。
func OpenWatchedFile(list []Runtime, path string) (name string, size int64, body io.ReadCloser, err error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", 0, nil, fmt.Errorf("路径为空")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", 0, nil, err
	}
	if !allowedPath(list, abs) {
		return "", 0, nil, fmt.Errorf("路径不在看守目录内")
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", 0, nil, err
	}
	if info.IsDir() {
		return "", 0, nil, fmt.Errorf("不能下载目录")
	}
	file, err := os.Open(abs)
	if err != nil {
		return "", 0, nil, err
	}
	return filepath.Base(abs), info.Size(), file, nil
}

func allowedPath(list []Runtime, abs string) bool {
	for _, rt := range list {
		roots := []string{rt.Base, rt.Jar, rt.StaticDir, rt.BackupDir}
		for _, root := range roots {
			root = strings.TrimSpace(root)
			if root == "" {
				continue
			}
			absRoot, err := filepath.Abs(root)
			if err != nil {
				continue
			}
			// jar 配置是文件：允许该文件本身及其所在目录下同层文件。
			if info, err := os.Stat(absRoot); err == nil && !info.IsDir() {
				if abs == absRoot {
					return true
				}
				absRoot = filepath.Dir(absRoot)
			}
			rel, err := filepath.Rel(absRoot, abs)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				continue
			}
			return true
		}
	}
	return false
}
