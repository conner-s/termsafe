//go:build linux && (amd64 || arm64)

package termsafe

import (
	"errors"
	"strconv"
	"syscall"
	"unsafe"
)

const ioctlSetTermios = syscall.TCSETS

// openPTY opens a pseudo terminal and returns the descriptors of its two
// ends: the controlling one and the one a program is given.
func openPTY() (master, slave int, err error) {
	master, err = syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return -1, -1, err
	}
	var unlock int32
	var number uint32
	if !ioctl(master, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock)) || !ioctl(master, syscall.TIOCGPTN, unsafe.Pointer(&number)) {
		syscall.Close(master)
		return -1, -1, errors.New("the pseudo terminal could not be unlocked or numbered")
	}
	slave, err = syscall.Open("/dev/pts/"+strconv.Itoa(int(number)), syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		syscall.Close(master)
		return -1, -1, err
	}
	return master, slave, nil
}
