package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"fluxguard/server/plugin"
)

var fileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// PutFile 把文件交给这个插件。名字不能是路径，内容按 SHA-256 核对。
func PutFile(ctx context.Context, code, name string, body []byte) error {
	if !fileNameOK(name) {
		return fmt.Errorf("文件名无效")
	}
	if len(body) > maxFile {
		return fmt.Errorf("文件过大")
	}
	params := FilePutParams{Name: name, SHA256: fileSHA(body), Body: base64.StdEncoding.EncodeToString(body)}
	var res FileResult
	if err := plugin.Call(ctx, code, methodFilePut, params, "", &res); err != nil {
		return err
	}
	if res.Error != "" || !res.OK {
		if res.Error != "" {
			return fmt.Errorf("%s", res.Error)
		}
		return fmt.Errorf("插件应答无效")
	}
	return nil
}

// GetFile 按名字从插件取回文件。应答里的校验必须和内容一致。
func GetFile(ctx context.Context, code, name string) ([]byte, error) {
	if !fileNameOK(name) {
		return nil, fmt.Errorf("文件名无效")
	}
	var res FileResult
	if err := plugin.Call(ctx, code, methodFileGet, FileGetParams{Name: name}, "", &res); err != nil {
		return nil, err
	}
	if res.Error != "" {
		return nil, fmt.Errorf("%s", res.Error)
	}
	if res.Name != name {
		return nil, fmt.Errorf("插件应答无效")
	}
	body, err := base64.StdEncoding.DecodeString(res.Body)
	if err != nil || len(body) > maxFile {
		return nil, fmt.Errorf("插件应答无效")
	}
	if fileSHA(body) != res.SHA256 {
		return nil, fmt.Errorf("文件校验失败")
	}
	return body, nil
}

func fileNameOK(name string) bool {
	if strings.Contains(name, "..") {
		return false
	}
	return fileName.MatchString(name)
}

func fileSHA(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
