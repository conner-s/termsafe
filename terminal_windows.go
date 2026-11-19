//go:build windows

package termsafe

import (
	"os"
	"syscall"
	"unsafe"
)

// kernel32.dll is loaded in every Windows process.
var (
	kernel32                         = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleOutputCP           = kernel32.NewProc("GetConsoleOutputCP")
	procGetFileInformationByHandleEx = kernel32.NewProc("GetFileInformationByHandleEx")
)

// examineOutput gathers what Windows says about out and decides nothing:
// judge does that. It asks the questions package os asks in os.newFile, in
// the same order, so that "console" here is what os writes to with
// WriteConsoleW.
func examineOutput(out *os.File) outputFacts {
	if out == nil {
		return outputFacts{kind: outNone}
	}
	handle := syscall.Handle(out.Fd())

	fileType, err := syscall.GetFileType(handle)
	if err != nil || fileType == syscall.FILE_TYPE_CHAR {
		var mode uint32
		if syscall.GetConsoleMode(handle, &mode) == nil {
			codePage, _, _ := procGetConsoleOutputCP.Call()
			return outputFacts{kind: outConsole, consoleMode: mode, codePage: uint32(codePage)}
		}
	}
	if err != nil {
		return outputFacts{kind: outNone}
	}
	if fileType == syscall.FILE_TYPE_PIPE {
		return outputFacts{kind: outPipe, pipeName: pipeName(handle)}
	}
	return outputFacts{kind: outFile}
}

// pipeName returns the name of a pipe, or "" when Windows does not give it.
// FILE_NAME_INFO is a length in bytes followed by the name in UTF-16.
func pipeName(handle syscall.Handle) string {
	const fileNameInfo = 2
	var buf struct {
		length uint32
		name   [syscall.MAX_PATH]uint16
	}
	ok, _, _ := procGetFileInformationByHandleEx.Call(
		uintptr(handle), fileNameInfo, uintptr(unsafe.Pointer(&buf)), unsafe.Sizeof(buf))
	if ok == 0 {
		return ""
	}
	n := int(buf.length / 2)
	if n > len(buf.name) {
		n = len(buf.name)
	}
	return syscall.UTF16ToString(buf.name[:n])
}
