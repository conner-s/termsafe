//go:build (linux && (amd64 || arm64)) || darwin

package termsafe

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// pipeTerminal is an emulator at the far end of two pipes, in real time: the
// probe's own reading and writing with deadlines is under test here, where
// TestAsk scripts it away.
type pipeTerminal struct {
	*emulator
	tty      fdTTY
	toTerm   *os.File // the program's end: what it writes
	fromTerm *os.File // the program's end: what it reads
	termIn   *os.File // the terminal's end: what the program wrote
	termOut  *os.File // the terminal's end: its answers
	delay    time.Duration
	mu       sync.Mutex
	got      []byte
	done     chan struct{}
}

// newPipeTerminal starts the terminal. answer, when not nil, replaces its
// true reports, as in emulator.
func newPipeTerminal(t *testing.T, decodesUTF8 bool, delay time.Duration, answer func(int, string) string) *pipeTerminal {
	t.Helper()
	termIn, toTerm, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	fromTerm, termOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	p := &pipeTerminal{
		emulator: newEmulator(decodesUTF8),
		toTerm:   toTerm, fromTerm: fromTerm, termIn: termIn, termOut: termOut,
		delay: delay,
		done:  make(chan struct{}),
	}
	p.answer = answer
	in, _ := descriptor(fromTerm)
	out, _ := descriptor(toTerm)
	p.tty = fdTTY{in: in, out: out}
	go p.run()
	t.Cleanup(func() {
		toTerm.Close()
		<-p.done
		for _, f := range []*os.File{fromTerm, termIn, termOut} {
			f.Close()
		}
	})
	return p
}

// run answers until the program closes its end.
func (p *pipeTerminal) run() {
	defer close(p.done)
	buf := make([]byte, 256)
	for {
		n, err := p.termIn.Read(buf)
		if err != nil {
			return
		}
		p.mu.Lock()
		p.got = append(p.got, buf[:n]...)
		reply := p.feed(buf[:n])
		p.mu.Unlock()
		if len(reply) > 0 {
			time.Sleep(p.delay)
			_, _ = p.termOut.Write(reply)
		}
	}
}

// sent returns what the terminal has been sent so far.
func (p *pipeTerminal) sent() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return string(p.got)
}

// settled waits for the terminal to have taken in everything the probe
// wrote, and reports whether its screen is as it was.
func (p *pipeTerminal) settled(wantSuffix string) bool {
	for i := 0; i < 200; i++ {
		p.mu.Lock()
		ok := strings.HasSuffix(string(p.got), wantSuffix) && p.untouched()
		p.mu.Unlock()
		if ok {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func TestAsk_OverDescriptors(t *testing.T) {
	const restore = "\x1b[7;12H"
	t.Run("a terminal that decodes UTF-8", func(t *testing.T) {
		p := newPipeTerminal(t, true, 0, nil)
		got, finding := ask(p.tty, 80, time.Now().Add(2*time.Second))
		if got != EncodingUTF8 {
			t.Errorf("verdict %v (%s), want UTF-8", got, finding)
		}
		if !p.settled(restore) {
			t.Errorf("the screen is not as it was; the terminal got %q", p.sent())
		}
	})
	t.Run("an 8-bit terminal", func(t *testing.T) {
		p := newPipeTerminal(t, false, 0, nil)
		got, finding := ask(p.tty, 80, time.Now().Add(2*time.Second))
		if got != EncodingNotUTF8 {
			t.Errorf("verdict %v (%s), want not UTF-8", got, finding)
		}
		if !p.settled(restore) {
			t.Errorf("the screen is not as it was; the terminal got %q", p.sent())
		}
	})
	t.Run("a terminal that never answers: back at the deadline", func(t *testing.T) {
		p := newPipeTerminal(t, true, 0, func(int, string) string { return "" })
		const timeout = 150 * time.Millisecond
		start := time.Now()
		got, finding := ask(p.tty, 80, start.Add(timeout))
		elapsed := time.Since(start)
		if got != EncodingUnknown || !strings.Contains(finding, "did not answer in time") {
			t.Errorf("verdict %v, finding %q", got, finding)
		}
		if elapsed < timeout || elapsed > timeout+time.Second {
			t.Errorf("returned after %v with a timeout of %v", elapsed, timeout)
		}
		if !p.settled(cursorQuery) {
			t.Errorf("the terminal got %q, want the one query and nothing drawn", p.sent())
		}
	})
	t.Run("a terminal that answers late: back at the deadline, the answer still unread", func(t *testing.T) {
		p := newPipeTerminal(t, true, 400*time.Millisecond, nil)
		const timeout = 100 * time.Millisecond
		start := time.Now()
		got, _ := ask(p.tty, 80, start.Add(timeout))
		elapsed := time.Since(start)
		if got != EncodingUnknown {
			t.Errorf("verdict %v, want cannot tell", got)
		}
		if elapsed > 350*time.Millisecond {
			t.Errorf("returned after %v with a timeout of %v: it waited for the late answer", elapsed, timeout)
		}
		// The answer arrives afterwards and is the program's to read.
		buf := make([]byte, 32)
		n, err := p.tty.read(buf, time.Now().Add(2*time.Second))
		if err != nil || string(buf[:n]) != "\x1b[7;12R" {
			t.Errorf("after the deadline the input holds %q (%v), want the late report", buf[:n], err)
		}
	})
	t.Run("a terminal that answers with garbage", func(t *testing.T) {
		p := newPipeTerminal(t, true, 0, func(int, string) string { return strings.Repeat("\x00\xff[;R", 60) })
		got, _ := ask(p.tty, 80, time.Now().Add(2*time.Second))
		if got != EncodingUnknown {
			t.Errorf("verdict %v, want cannot tell", got)
		}
		if !p.settled(cursorQuery) {
			t.Errorf("the terminal got %q, want the one query and nothing drawn", p.sent())
		}
	})
}

func TestFdTTY(t *testing.T) {
	soon := func() time.Time { return time.Now().Add(100 * time.Millisecond) }

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rfd, _ := descriptor(r)
	wfd, _ := descriptor(w)

	// Nothing to read: back at the deadline, and at once when it has passed.
	start := time.Now()
	if _, err := (fdTTY{in: rfd}).read(make([]byte, 8), soon()); err != errDeadline {
		t.Errorf("reading an empty pipe: %v, want the deadline", err)
	}
	if elapsed := time.Since(start); elapsed < 90*time.Millisecond || elapsed > time.Second {
		t.Errorf("an empty read took %v with a deadline 100ms off", elapsed)
	}
	start = time.Now()
	if _, err := (fdTTY{in: rfd}).read(make([]byte, 8), time.Now().Add(-time.Hour)); err != errDeadline {
		t.Errorf("reading past the deadline: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("a read past its deadline took %v", elapsed)
	}

	// What is there is read even past the deadline: it does not block.
	if err := (fdTTY{out: wfd}).write([]byte("abc"), soon()); err != nil {
		t.Fatalf("writing to a pipe: %v", err)
	}
	buf := make([]byte, 8)
	if n, err := (fdTTY{in: rfd}).read(buf, time.Now().Add(-time.Hour)); err != nil || string(buf[:n]) != "abc" {
		t.Errorf("read %q, %v; want abc", buf[:n], err)
	}

	// A pipe that is full cannot be written to: back at the deadline.
	if err := syscall.SetNonblock(wfd, true); err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := syscall.Write(wfd, make([]byte, 4096)); err != nil {
			break
		}
	}
	if err := (fdTTY{out: wfd}).write(make([]byte, 65536), soon()); err != errDeadline {
		t.Errorf("writing to a full pipe: %v, want the deadline", err)
	}

	// The other end gone: the input has ended, and writing fails.
	w.Close()
	for {
		if _, err := (fdTTY{in: rfd}).read(make([]byte, 65536), soon()); err != nil {
			if err != io.EOF {
				t.Errorf("reading a closed pipe: %v, want the end of the input", err)
			}
			break
		}
	}
	r2, w2, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()
	w2fd, _ := descriptor(w2)
	r2.Close()
	if err := (fdTTY{out: w2fd}).write([]byte("x"), soon()); err == nil || err == errDeadline {
		t.Errorf("writing to a pipe nobody reads: %v, want an error of the system", err)
	}
	r.Close()

	// Descriptors select cannot take.
	if waitFD(-1, false, soon()) || waitFD(1<<20, true, soon()) {
		t.Error("waitFD says a descriptor that cannot exist is ready")
	}
}

// TestProbe_NotATerminal: given anything that is not a terminal, Probe writes
// nothing, reads nothing and says why. That it writes nothing is seen from
// the pipe and the file staying empty.
func TestProbe_NotATerminal(t *testing.T) {
	before := judge(outputFacts{kind: outNotTerminal}, env{"LANG": "en_US.UTF-8"}.get)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	file, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	closed, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()

	const (
		notTerminal = "the terminal was not asked: the input and the output are not both a terminal"
		noFiles     = "the terminal was not asked: it was not given an open input and output"
	)
	for _, tc := range []struct {
		name    string
		in, out *os.File
		want    string
	}{
		{"pipes", r, w, notTerminal},
		{"a file", file, file, notTerminal},
		{"the null device, a character device that is no terminal", null, null, notTerminal},
		{"nil", nil, nil, noFiles},
		{"nil input", nil, w, noFiles},
		{"nil output", r, nil, noFiles},
		{"a closed file", closed, closed, noFiles},
	} {
		start := time.Now()
		got := before.Probe(tc.in, tc.out, 5*time.Second)
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Errorf("%s: Probe took %v to decide not to ask", tc.name, elapsed)
		}
		if got.Encoding != before.Encoding || got.Reason != before.Reason {
			t.Errorf("%s: the verdict changed to %v, %q", tc.name, got.Encoding, got.Reason)
		}
		if last := got.Evidence[len(got.Evidence)-1].String(); last != tc.want {
			t.Errorf("%s: evidence %q, want %q", tc.name, last, tc.want)
		}
	}

	// Nothing was written to the pipe or the file.
	rfd, _ := descriptor(r)
	if waitFD(rfd, false, time.Now()) {
		t.Error("Probe wrote to a pipe")
	}
	if info, err := file.Stat(); err != nil || info.Size() != 0 {
		t.Errorf("Probe wrote to a file: size %d, %v", info.Size(), err)
	}
}
