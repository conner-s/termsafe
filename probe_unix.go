//go:build (linux && (amd64 || arm64)) || darwin

package termsafe

import (
	"io"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// openTTY looks at what in and out are and reads the terminal's mode and
// size. It changes nothing. The tty it returns is used only when the refusal
// is empty.
func openTTY(in, out *os.File) (tty, int, string) {
	inFD, haveIn := descriptor(in)
	outFD, haveOut := descriptor(out)
	st := examineTTY(inFD, outFD)
	return fdTTY{in: inFD, out: outFD}, st.columns, refusal(haveIn && haveOut, st)
}

// descriptor returns the file descriptor of f without changing the file, as
// f.Fd would by switching it to blocking mode.
func descriptor(f *os.File) (fd int, ok bool) {
	fd = -1
	conn, err := f.SyscallConn()
	if err == nil {
		err = conn.Control(func(d uintptr) { fd = int(d) })
	}
	return fd, err == nil
}

// examineTTY asks the terminal driver for the mode of in and out and the
// size of out. A descriptor that is not a terminal fails the first request.
func examineTTY(in, out int) ttyState {
	var inMode, outMode syscall.Termios
	var size struct{ rows, columns, width, height uint16 }
	st := ttyState{
		inTTY:  ioctl(in, ioctlGetTermios, unsafe.Pointer(&inMode)),
		outTTY: ioctl(out, ioctlGetTermios, unsafe.Pointer(&outMode)),
	}
	ioctl(out, syscall.TIOCGWINSZ, unsafe.Pointer(&size))
	st.raw = inMode.Lflag&(lflagCanonical|lflagEcho) == 0
	st.columns = int(size.columns)
	return st
}

func ioctl(fd int, request uint, arg unsafe.Pointer) bool {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(request), uintptr(arg))
	return errno == 0
}

// fdTTY is a terminal by its two file descriptors. It waits with select and
// reads or writes only when that will not block, so it never holds a read
// open past the deadline for the program's later input to fall into.
type fdTTY struct{ in, out int }

func (t fdTTY) write(b []byte, deadline time.Time) error {
	for len(b) > 0 {
		if !waitFD(t.out, true, deadline) {
			return errDeadline
		}
		n, err := syscall.Write(t.out, b)
		if err != nil && err != syscall.EINTR && err != syscall.EAGAIN {
			return err
		}
		b = b[max(n, 0):]
	}
	return nil
}

func (t fdTTY) read(buf []byte, deadline time.Time) (int, error) {
	n, err := 0, error(syscall.EINTR)
	for err == syscall.EINTR || err == syscall.EAGAIN {
		if !waitFD(t.in, false, deadline) {
			return 0, errDeadline
		}
		n, err = syscall.Read(t.in, buf)
	}
	if err != nil || n <= 0 {
		return 0, io.EOF
	}
	return n, nil
}

// waitFD waits until fd can be read, or written when forWrite is set, and
// reports whether it can. It waits no longer than the deadline; past the
// deadline it looks once and does not wait.
func waitFD(fd int, forWrite bool, deadline time.Time) bool {
	var set syscall.FdSet
	bits := int(unsafe.Sizeof(set.Bits[0])) * 8
	if fd < 0 || fd/bits >= len(set.Bits) {
		return false
	}
	err := error(syscall.EINTR)
	for err == syscall.EINTR {
		set = syscall.FdSet{}
		set.Bits[fd/bits] |= 1 << (uint(fd) % uint(bits))
		wait := syscall.NsecToTimeval(int64(max(time.Until(deadline), 0)))
		if forWrite {
			err = selectFD(fd+1, nil, &set, &wait)
		} else {
			err = selectFD(fd+1, &set, nil, &wait)
		}
	}
	return err == nil && set.Bits[fd/bits]&(1<<(uint(fd)%uint(bits))) != 0
}
