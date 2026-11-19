//go:build (linux && (amd64 || arm64)) || darwin

package termsafe

import (
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// These tests put Probe in front of a real terminal device: a pseudo
// terminal, with the kernel's line discipline between the two ends, and an
// emulator playing the terminal at the other end. openPTY and the request
// that sets a terminal's mode are in a file for each system.

// ptyTerminal is an emulator on the controlling end of a pseudo terminal.
type ptyTerminal struct {
	*emulator
	master int
	slave  *os.File // what a program would have as its standard input and output
	mu     sync.Mutex
	got    []byte
	stop   chan struct{}
	done   chan struct{}
}

func newPTYTerminal(t *testing.T, decodesUTF8 bool, answer func(int, string) string) *ptyTerminal {
	t.Helper()
	master, slave, err := openPTY()
	if err != nil {
		t.Skipf("no pseudo terminal to test with: %v", err)
	}
	size := struct{ rows, columns, width, height uint16 }{24, 80, 0, 0}
	if !ioctl(master, syscall.TIOCSWINSZ, unsafe.Pointer(&size)) {
		t.Skip("the pseudo terminal takes no size")
	}
	p := &ptyTerminal{
		emulator: newEmulator(decodesUTF8),
		master:   master,
		slave:    os.NewFile(uintptr(slave), "pty"),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	p.answer = answer
	go p.run()
	t.Cleanup(func() {
		close(p.stop)
		<-p.done
		p.slave.Close()
		syscall.Close(master)
	})
	return p
}

// raw switches line editing and echo off, as a TUI framework does when it
// starts. Probe itself never does.
func (p *ptyTerminal) raw(t *testing.T) {
	t.Helper()
	fd, _ := descriptor(p.slave)
	var mode syscall.Termios
	if !ioctl(fd, ioctlGetTermios, unsafe.Pointer(&mode)) {
		t.Fatal("cannot read the mode of the pseudo terminal")
	}
	mode.Lflag &^= lflagCanonical | lflagEcho
	if !ioctl(fd, ioctlSetTermios, unsafe.Pointer(&mode)) {
		t.Fatal("cannot set the mode of the pseudo terminal")
	}
}

func (p *ptyTerminal) run() {
	defer close(p.done)
	tty := fdTTY{in: p.master, out: p.master}
	buf := make([]byte, 256)
	for {
		select {
		case <-p.stop:
			return
		default:
		}
		n, err := tty.read(buf, time.Now().Add(20*time.Millisecond))
		if err != nil {
			continue // nothing yet
		}
		p.mu.Lock()
		p.got = append(p.got, buf[:n]...)
		reply := p.feed(buf[:n])
		p.mu.Unlock()
		if len(reply) > 0 {
			_ = tty.write(reply, time.Now().Add(time.Second))
		}
	}
}

// sent waits a moment for what the program wrote to arrive and returns it,
// with whether the screen is as it was.
func (p *ptyTerminal) sent(wantSuffix string) (string, bool) {
	for i := 0; ; i++ {
		p.mu.Lock()
		got, clean := string(p.got), p.untouched()
		p.mu.Unlock()
		if (strings.HasSuffix(got, wantSuffix) && clean) || i == 100 {
			return got, clean && strings.HasSuffix(got, wantSuffix)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestProbe_OnATerminal(t *testing.T) {
	saysUTF8 := judge(outputFacts{kind: outTerminal}, env{"LANG": "en_US.UTF-8"}.get)
	const restore = "\x1b[7;12H"

	t.Run("in raw mode, a terminal that decodes UTF-8", func(t *testing.T) {
		p := newPTYTerminal(t, true, nil)
		p.raw(t)
		got := Terminal{}.Probe(p.slave, p.slave, 2*time.Second)
		if got.Encoding != EncodingUTF8 {
			t.Errorf("Encoding = %v, want UTF-8; evidence %v", got.Encoding, got.Evidence)
		}
		if sent, clean := p.sent(restore); !clean {
			t.Errorf("the screen is not as it was; the terminal got %q", sent)
		}
	})

	t.Run("in raw mode, an 8-bit terminal: its answer outranks the environment", func(t *testing.T) {
		p := newPTYTerminal(t, false, nil)
		p.raw(t)
		got := saysUTF8.Probe(p.slave, p.slave, 2*time.Second)
		if got.Encoding != EncodingNotUTF8 {
			t.Errorf("Encoding = %v, want not UTF-8; evidence %v", got.Encoding, got.Evidence)
		}
		if !strings.HasSuffix(got.Reason.String(), "(the environment said: UTF-8)") {
			t.Errorf("Reason = %q", got.Reason.String())
		}
		if len(got.Evidence) != len(saysUTF8.Evidence)+1 {
			t.Errorf("evidence %v, want the environment's lines and one more", got.Evidence)
		}
		if sent, clean := p.sent(restore); !clean {
			t.Errorf("the screen is not as it was; the terminal got %q", sent)
		}
	})

	t.Run("in raw mode, a terminal that never answers: back at the deadline", func(t *testing.T) {
		p := newPTYTerminal(t, true, func(int, string) string { return "" })
		p.raw(t)
		const timeout = 150 * time.Millisecond
		start := time.Now()
		got := saysUTF8.Probe(p.slave, p.slave, timeout)
		if elapsed := time.Since(start); elapsed < timeout || elapsed > timeout+time.Second {
			t.Errorf("returned after %v with a timeout of %v", elapsed, timeout)
		}
		if got.Encoding != EncodingUTF8 || got.Reason != saysUTF8.Reason {
			t.Errorf("the environment's verdict did not stand: %v, %q", got.Encoding, got.Reason)
		}
		if sent, clean := p.sent(cursorQuery); !clean {
			t.Errorf("the terminal got %q, want the one query and nothing drawn", sent)
		}
	})

	// A terminal as a program finds it when it starts, before any TUI
	// framework has switched echo and line editing off. Asking would have
	// the terminal driver draw the answer on the screen. Probe sees the mode
	// and writes nothing.
	t.Run("not in raw mode: nothing is written", func(t *testing.T) {
		p := newPTYTerminal(t, true, nil)
		got := saysUTF8.Probe(p.slave, p.slave, 2*time.Second)
		want := "the terminal was not asked: the terminal is not in raw mode, so it would draw the answer and hold it back until a newline"
		if last := got.Evidence[len(got.Evidence)-1].String(); last != want {
			t.Errorf("evidence %q, want %q", last, want)
		}
		if got.Encoding != saysUTF8.Encoding {
			t.Errorf("the verdict changed to %v", got.Encoding)
		}
		time.Sleep(60 * time.Millisecond)
		if sent, _ := p.sent(""); sent != "" {
			t.Errorf("the terminal was sent %q", sent)
		}
	})
}
