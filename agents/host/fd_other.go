//go:build !linux && !windows && !darwin

package host

func fileDescriptorLimits() (allocated, maximum uint64, ok bool) {
	return 0, 0, false
}
