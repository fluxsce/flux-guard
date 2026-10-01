//go:build linux

package host

import "os"

func fileDescriptorLimits() (allocated, maximum uint64, ok bool) {
	allocated, maximum, ok = readFileNr()
	// 部分内核 / 容器里 file-nr 的已分配列停在 0，空闲列也恒为 0。
	// 这时改计各进程打开的文件描述符，避免界面一直显示 0。
	if allocated == 0 {
		if counted, seen := countProcFDs(); seen && counted > 0 {
			allocated = counted
			ok = true
		}
	}
	if !ok {
		return 0, 0, false
	}
	return allocated, maximum, true
}

func readFileNr() (allocated, maximum uint64, ok bool) {
	body, err := os.ReadFile("/proc/sys/fs/file-nr")
	if err != nil {
		return 0, 0, false
	}
	return parseFileNr(string(body))
}

func countProcFDs() (uint64, bool) {
	proc, err := os.Open("/proc")
	if err != nil {
		return 0, false
	}
	defer proc.Close()
	names, err := proc.Readdirnames(-1)
	if err != nil {
		return 0, false
	}
	var total uint64
	var seen bool
	for _, name := range names {
		if !numericName(name) {
			continue
		}
		n, err := countNames("/proc/" + name + "/fd")
		if err != nil {
			continue
		}
		seen = true
		total += n
	}
	return total, seen
}

func numericName(name string) bool {
	if name == "" || name[0] < '1' || name[0] > '9' {
		return false
	}
	for i := 1; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	return true
}

func countNames(path string) (uint64, error) {
	dir, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return 0, err
	}
	return uint64(len(names)), nil
}
