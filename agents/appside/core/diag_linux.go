//go:build linux

package core

import (
	"os"
	"strconv"
	"syscall"
)

func init() {
	quitProcess = func(pid int) error {
		if pid <= 1 {
			return errQuitUnsupported
		}
		return syscall.Kill(pid, syscall.SIGQUIT)
	}
}

func findJVMsOS(arg string) ([]jvmProc, error) {
	self := os.Getpid()
	var out []jvmProc
	for _, pid := range pidsWithExactArg(arg) {
		if pid <= 1 || pid == self {
			continue
		}
		exe, _ := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
		out = append(out, jvmProc{PID: pid, Exe: exe})
	}
	return out, nil
}
