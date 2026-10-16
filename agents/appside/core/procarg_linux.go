//go:build linux

package core

import (
	"os"
	"strconv"
	"syscall"
)

func processHasExactArg(arg string) (bool, bool) {
	return len(pidsWithExactArg(arg)) > 0, true
}

func killExactArg(arg string) bool {
	self := os.Getpid()
	for _, pid := range pidsWithExactArg(arg) {
		if pid <= 1 || pid == self {
			continue
		}
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	return true
}

func pidsWithExactArg(arg string) []int {
	if arg == "" {
		return nil
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	self := os.Getpid()
	var pids []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 || pid == self {
			continue
		}
		buf, err := os.ReadFile("/proc/" + entry.Name() + "/cmdline")
		if err != nil || len(buf) == 0 {
			continue
		}
		if cmdlineHasExactArg(buf, arg) {
			pids = append(pids, pid)
		}
	}
	return pids
}
