//go:build darwin

package termsafe

import "syscall"

// The request that reads a terminal's mode, and the two flags of it that
// Probe looks at.
const (
	ioctlGetTermios = syscall.TIOCGETA
	lflagCanonical  = syscall.ICANON
	lflagEcho       = syscall.ECHO
)

func selectFD(n int, r, w *syscall.FdSet, timeout *syscall.Timeval) error {
	return syscall.Select(n, r, w, nil, timeout)
}
