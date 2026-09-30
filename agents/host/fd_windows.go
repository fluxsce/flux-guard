//go:build windows

package host

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	psapi                  = windows.NewLazySystemDLL("psapi.dll")
	procGetPerformanceInfo = psapi.NewProc("GetPerformanceInfo")
)

// performanceInformation 对齐 Windows PERFORMANCE_INFORMATION。
// uint32 与 uintptr 之间的对齐由编译器补齐，和 C 结构一致。
type performanceInformation struct {
	cb                uint32
	commitTotal       uintptr
	commitLimit       uintptr
	commitPeak        uintptr
	physicalTotal     uintptr
	physicalAvailable uintptr
	systemCache       uintptr
	kernelTotal       uintptr
	kernelPaged       uintptr
	kernelNonpaged    uintptr
	pageSize          uintptr
	handleCount       uint32
	processCount      uint32
	threadCount       uint32
}

func fileDescriptorLimits() (allocated, maximum uint64, ok bool) {
	var info performanceInformation
	info.cb = uint32(unsafe.Sizeof(info))
	r, _, _ := procGetPerformanceInfo.Call(uintptr(unsafe.Pointer(&info)), uintptr(info.cb))
	if r == 0 {
		return 0, 0, false
	}
	return uint64(info.handleCount), 0, true
}
