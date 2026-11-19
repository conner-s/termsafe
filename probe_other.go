//go:build !((linux && (amd64 || arm64)) || darwin)

package termsafe

import "os"

// openTTY refuses: reading a terminal's mode and waiting on it with a
// deadline take system calls this package has only for Linux on amd64 and
// arm64 and for macOS.
func openTTY(in, out *os.File) (tty, int, string) {
	return nil, 0, "asking the terminal is not implemented on this system"
}
