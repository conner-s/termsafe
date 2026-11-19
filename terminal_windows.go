//go:build windows

package termsafe

import (
	"os"
	"syscall"
)

// kernel32.dll is loaded in every Windows process.
var procGetConsoleOutputCP = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleOutputCP")

// outputKind says whether out is a console and, if so, which code page the
// console decodes bytes with. GetConsoleMode succeeds on a console handle and
// on nothing else; it is how package os decides to write with WriteConsoleW.
func outputKind(out *os.File) (output, uint32) {
	if out == nil {
		return outNone, 0
	}
	var mode uint32
	if syscall.GetConsoleMode(syscall.Handle(out.Fd()), &mode) != nil {
		return outNotConsole, 0
	}
	codePage, _, _ := procGetConsoleOutputCP.Call()
	return outConsole, uint32(codePage)
}
