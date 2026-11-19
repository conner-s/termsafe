//go:build windows

package termsafe

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
	"unsafe"
)

// The tests in this file had run on no machine when they were written: the
// module is developed on macOS. They compile and pass go vet for windows.
// The workflow on the GitHub mirror runs them on a Windows runner, after a
// merge, and reports which of them ran, passed or skipped. To turn what judge takes from Go's source into
// something seen on a real console window:
//
//	go test -run Windows -v .

var (
	procGetConsoleScreenBufferInfo  = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procReadConsoleOutputCharacterW = kernel32.NewProc("ReadConsoleOutputCharacterW")
	procSetConsoleCursorPosition    = kernel32.NewProc("SetConsoleCursorPosition")
	procSetConsoleOutputCP          = kernel32.NewProc("SetConsoleOutputCP")
	procAllocConsole                = kernel32.NewProc("AllocConsole")
)

// coord is COORD; screenBufferInfo is CONSOLE_SCREEN_BUFFER_INFO.
type coord struct{ x, y int16 }

type screenBufferInfo struct {
	size       coord
	cursor     coord
	attributes uint16
	window     [4]int16
	maxWindow  coord
}

// packed is a COORD as Windows takes it by value: x in the low word.
func (c coord) packed() uintptr {
	return uintptr(uint16(c.x)) | uintptr(uint16(c.y))<<16
}

// TestWindowsExamineOutput: what examineOutput reports for the kinds of
// handle a test can make without a console.
func TestWindowsExamineOutput(t *testing.T) {
	if kind := examineOutput(nil).kind; kind != outNone {
		t.Errorf("nil: kind %d, want outNone", kind)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if facts := examineOutput(w); facts.kind != outPipe || isPTYName(facts.pipeName) {
		t.Errorf("a pipe: %+v, want outPipe with a name that is no pty's", facts)
	}

	file, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if kind := examineOutput(file).kind; kind != outFile {
		t.Errorf("a file: kind %d, want outFile", kind)
	}

	// NUL is a character device, as a console is, and GetConsoleMode
	// refuses it.
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	if kind := examineOutput(null).kind; kind != outFile {
		t.Errorf("%s: kind %d, want outFile", os.DevNull, kind)
	}

	// Under go test the standard output is a pipe to the go command. From
	// mintty or Git Bash it may be a pty: the name shows which.
	t.Logf("standard output of this test: %+v", examineOutput(os.Stdout))
	t.Logf("CheckTerminal(os.Stdout): %v, %s", CheckTerminal(os.Stdout).Encoding, CheckTerminal(os.Stdout).Reason)
}

// TestWindowsConsoleReceivesCharacters is the evidence for the console rows
// of judge. It sets the console to code page 437, writes three characters
// through an *os.File, and reads the cells back from the screen buffer. If
// package os wrote bytes, code page 437 would turn each character into two
// box-drawing and accented ones; the third character is not in that code
// page at all. They come back as themselves only if they went through
// WriteConsoleW.
func TestWindowsConsoleReceivesCharacters(t *testing.T) {
	console, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		// A process that was started without a console, as on a CI
		// runner, can ask Windows for one. It has no window anybody
		// sees, but it has a screen buffer, which is all this needs.
		procAllocConsole.Call()
		console, err = os.OpenFile("CONOUT$", os.O_RDWR, 0)
	}
	if err != nil {
		t.Skipf("no console to write to, and none to be had: %v", err)
	}
	defer console.Close()

	facts := examineOutput(console)
	if facts.kind != outConsole {
		t.Fatalf("CONOUT$: %+v, want a console", facts)
	}
	t.Logf("the console: %+v", facts)
	if got := judge(facts, os.Getenv); got.Encoding != EncodingUTF8 {
		t.Errorf("judge: %v, want UTF-8", got.Encoding)
	}

	handle := uintptr(console.Fd())
	procSetConsoleOutputCP.Call(437)
	defer procSetConsoleOutputCP.Call(uintptr(facts.codePage))

	var info screenBufferInfo
	if ok, _, err := procGetConsoleScreenBufferInfo.Call(handle, uintptr(unsafe.Pointer(&info))); ok == 0 {
		t.Fatalf("GetConsoleScreenBufferInfo: %v", err)
	}
	start := info.cursor

	// U+00E4 and U+00DB are in code page 437 as single bytes, but their
	// UTF-8 encodings are two bytes each, and that of U+00DB holds 0x9B.
	// U+0416 is not in code page 437.
	sent := []rune{0x00E4, 0x00DB, 0x0416}
	if _, err := console.WriteString(string(sent)); err != nil {
		t.Fatalf("writing to the console: %v", err)
	}

	cells := make([]uint16, len(sent))
	var read uint32
	ok, _, readErr := procReadConsoleOutputCharacterW.Call(handle,
		uintptr(unsafe.Pointer(&cells[0])), uintptr(len(cells)), start.packed(), uintptr(unsafe.Pointer(&read)))

	// Put the line back as it was before judging.
	procSetConsoleCursorPosition.Call(handle, start.packed())
	_, _ = console.WriteString("   ")
	procSetConsoleCursorPosition.Call(handle, start.packed())

	if ok == 0 {
		t.Fatalf("ReadConsoleOutputCharacterW: %v", readErr)
	}
	if got := utf16.Decode(cells[:read]); string(got) != string(sent) {
		t.Errorf("the console holds %U where %U was written through an *os.File", got, sent)
	}
}
