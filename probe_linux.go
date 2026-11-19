//go:build linux && (amd64 || arm64)

package termsafe

import "syscall"

// The request that reads a terminal's mode, and the two flags of it that
// Probe looks at. Package syscall does not name the flags on every Linux
// architecture; these are their values on amd64 and arm64.
const (
	ioctlGetTermios = syscall.TCGETS
	lflagCanonical  = 0x2 // ICANON
	lflagEcho       = 0x8 // ECHO
)

func selectFD(n int, r, w *syscall.FdSet, timeout *syscall.Timeval) error {
	_, err := syscall.Select(n, r, w, nil, timeout)
	return err
}
