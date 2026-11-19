//go:build !windows

package termsafe

import "os"

// outputKind says whether out is a character device. A terminal is one, and
// so is /dev/null: telling them apart takes an ioctl that the standard
// library does not offer portably, and the answer only goes into the
// evidence.
func outputKind(out *os.File) (output, uint32) {
	if out == nil {
		return outNone, 0
	}
	info, err := out.Stat()
	if err != nil {
		return outNone, 0
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return outTerminal, 0
	}
	return outNotTerminal, 0
}
