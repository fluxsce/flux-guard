//go:build darwin

package host

import "golang.org/x/sys/unix"

func fileDescriptorLimits() (allocated, maximum uint64, ok bool) {
	n, err := unix.SysctlUint32("kern.num_files")
	if err != nil {
		return 0, 0, false
	}
	m, err := unix.SysctlUint32("kern.maxfiles")
	if err != nil {
		return uint64(n), 0, true
	}
	return uint64(n), uint64(m), true
}
