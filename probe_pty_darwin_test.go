//go:build darwin

package termsafe

import (
	"errors"
	"syscall"
	"unsafe"
)

const ioctlSetTermios = syscall.TIOCSETA

// openPTY opens a pseudo terminal and returns the descriptors of its two
// ends: the controlling one and the one a program is given.
func openPTY() (master, slave int, err error) {
	master, err = syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return -1, -1, err
	}
	var name [128]byte
	if !ioctl(master, syscall.TIOCPTYGRANT, nil) || !ioctl(master, syscall.TIOCPTYUNLK, nil) ||
		!ioctl(master, syscall.TIOCPTYGNAME, unsafe.Pointer(&name)) {
		syscall.Close(master)
		return -1, -1, errors.New("the pseudo terminal could not be unlocked or named")
	}
	n := 0
	for n < len(name) && name[n] != 0 {
		n++
	}
	slave, err = syscall.Open(string(name[:n]), syscall.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		syscall.Close(master)
		return -1, -1, err
	}
	return master, slave, nil
}
