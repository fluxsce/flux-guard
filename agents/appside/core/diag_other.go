//go:build !linux && !windows

package core

import (
	"context"
	"fmt"
	"time"
)

func findJVMsOS(arg string) ([]jvmProc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := runCommand(ctx, 15*time.Second, "jcmd", "-l")
	if isNotFound(err) {
		return nil, fmt.Errorf("未找到 jcmd，无法定位 Tomcat 进程")
	}
	if err != nil {
		return nil, fmt.Errorf("jcmd -l 失败：%s", clipDiag(joinOut(out, err), 200))
	}
	return parseJcmdList(string(out), arg), nil
}
