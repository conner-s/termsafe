package termsafe

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// Probe asks the terminal itself whether it decodes UTF-8, and returns c with
// the answer added. It is for a program that owns the terminal, and it is
// never done unless the program calls it: [CheckTerminal] only reads the
// environment.
//
// in and out are the terminal's input and output as the program already
// holds them, os.Stdin and os.Stdout for most. Probe opens nothing and
// changes no terminal mode. It writes a cursor position query, then one
// character of two UTF-8 bytes and a second query, and compares the two
// answers: a terminal that decodes UTF-8 has moved one cell, any other has
// moved two. Then it blanks the cell or cells it drew and puts the cursor
// back, so nothing of it stays on the screen. The whole exchange ends at the
// timeout, answered or not; a fifth of a second is ample for a local
// terminal, and a slow ssh link wants more.
//
// An answer outranks the environment: Encoding becomes what the terminal
// showed, Reason says so and names what the environment had said, and
// Evidence keeps every line of the environment and gains one for the probe.
// Without an answer Encoding and Reason stay as they were, and the new line
// of Evidence says why there was none.
//
// Probe asks nothing, and says so in the evidence, unless all of this holds:
//
//   - in and out are both a terminal. Given a pipe, a file or nil it is a
//     no-op.
//   - The terminal is in raw mode: no line editing and no echo. In any other
//     mode the terminal driver would draw the answer on the screen and hold
//     it back until the next newline. Probe reads the mode and does not set
//     it: a TUI framework sets it when it starts, and before that the program
//     sets and restores it itself.
//   - The terminal's width is known and the cursor is not within two cells
//     of the right edge.
//   - The system is Linux on amd64 or arm64, or macOS. On Windows there is
//     nothing to ask where it matters: see [CheckTerminal].
//
// Two things a caller should know. Whatever else arrives on the input while
// Probe waits, a key the user pressed early, is read and dropped. And an
// answer that comes after the timeout is still on its way: the program will
// read it later as input. A TUI framework takes it for the cursor position
// report it is and ignores it. Call Probe before the user is expected to
// type, with a timeout the terminal can meet.
func (c Terminal) Probe(in, out *os.File, timeout time.Duration) Terminal {
	t, columns, refusal := openTTY(in, out)
	return c.probeWith(t, columns, refusal, timeout)
}

// probeWith is Probe after the operating system has been asked what in and
// out are.
func (c Terminal) probeWith(t tty, columns int, refusal string, timeout time.Duration) Terminal {
	if refusal == "" && timeout <= 0 {
		refusal = "no time was allowed for an answer"
	}
	if refusal != "" {
		return c.withProbe(EncodingUnknown, "the terminal was not asked: "+refusal)
	}
	return c.withProbe(ask(t, columns, time.Now().Add(timeout)))
}

// withProbe returns c with the probe's finding added to the evidence and,
// when the probe decided, in place of the environment's verdict.
func (c Terminal) withProbe(encoding Encoding, finding string) Terminal {
	evidence := make([]Text, len(c.Evidence), len(c.Evidence)+1)
	copy(evidence, c.Evidence)
	said := c.Encoding
	c.Evidence = append(evidence, ownWords(finding))
	if encoding != EncodingUnknown {
		c.Encoding = encoding
		c.Reason = ownWords(fmt.Sprintf("%s (the environment said: %s)", finding, said))
	}
	return c
}

// ttyState is what the operating system says about the files given to Probe.
type ttyState struct {
	inTTY, outTTY bool // each is a terminal
	raw           bool // the input has line editing and echo off
	columns       int  // the width of the terminal, 0 when it is not known
}

// refusal says why the terminal cannot be asked, or "" when it can.
func refusal(haveFiles bool, st ttyState) string {
	switch {
	case !haveFiles:
		return "it was not given an open input and output"
	case !st.inTTY || !st.outTTY:
		return "the input and the output are not both a terminal"
	case !st.raw:
		return "the terminal is not in raw mode, so it would draw the answer and hold it back until a newline"
	case st.columns < 4:
		return "the width of the terminal is not known"
	}
	return ""
}

// tty is the terminal as the probe uses it: bytes out and bytes in, each with
// a deadline. The real one is two file descriptors; the tests script others.
type tty interface {
	// write sends b, or gives up when it cannot be sent by the deadline.
	write(b []byte, deadline time.Time) error
	// read returns what the terminal has sent, waiting for it no longer
	// than the deadline.
	read(buf []byte, deadline time.Time) (int, error)
}

// errDeadline is what a tty returns when the deadline passed first.
var errDeadline = errors.New("termsafe: the terminal did not answer in time")

const (
	// cursorQuery is DSR 6, "report the cursor position". The answer is
	// ESC [ row ; column R.
	cursorQuery = "\x1b[6n"
	// probeCharacter is U+00E4 in UTF-8, C3 A4. A terminal that decodes
	// UTF-8 draws one narrow character; Unicode gives it no East Asian
	// "ambiguous" width, so no terminal draws it wide. Both bytes are
	// printable, and neither is a control, in the 8-bit character sets
	// (two characters, two cells) and they are one double-width character
	// in the East Asian double-byte ones (two cells again).
	probeCharacter = "\xc3\xa4"
)

// ask runs the exchange and judges it. Nothing is drawn before the terminal
// has answered once, and what is drawn is blanked however the rest goes.
func ask(t tty, columns int, deadline time.Time) (Encoding, string) {
	if t.write([]byte(cursorQuery), deadline) != nil {
		return EncodingUnknown, "the terminal was asked and could not be written to in time"
	}
	row, col, answered := readPosition(t, deadline)
	if !answered {
		return EncodingUnknown, "the terminal was asked for the cursor position and did not answer in time"
	}
	if col+2 > columns {
		return EncodingUnknown, "the terminal was not asked: the cursor is within two cells of the right edge"
	}

	sent := t.write([]byte(probeCharacter+cursorQuery), deadline) == nil
	row2, col2 := 0, 0
	if sent {
		row2, col2, answered = readPosition(t, deadline)
	}
	moved := col2 - col
	if !sent || !answered || row2 != row {
		moved = 0
	}

	// Blank what was drawn: as many cells as the cursor moved, or two when
	// that is not known. The deadline may have passed; a write that can go
	// out at once still does.
	blanks := "  "
	if moved == 1 {
		blanks = " "
	}
	erased := t.write([]byte(fmt.Sprintf("\x1b[%d;%dH%s\x1b[%d;%dH", row, col, blanks, row, col)), deadline) == nil

	finding := "the terminal was asked: it "
	switch {
	case !sent:
		return EncodingUnknown, finding + "answered once and could then not be written to in time"
	case !erased:
		// The verdict would stand, but the screen was left changed:
		// say that rather than anything else.
		return EncodingUnknown, finding + "took the test character and could then not be written to, so the character may still be drawn"
	case !answered:
		return EncodingUnknown, finding + "answered the first cursor position query and not the second in time"
	case moved == 1:
		return EncodingUTF8, finding + "drew a character of two UTF-8 bytes in one cell, as a terminal that decodes UTF-8 does"
	case moved == 2:
		return EncodingNotUTF8, finding + "drew a character of two UTF-8 bytes in two cells, as a terminal that does not decode UTF-8 does"
	}
	return EncodingUnknown, finding + fmt.Sprintf("moved the cursor from row %d column %d to row %d column %d, which tells nothing", row, col, row2, col2)
}

// readPosition reads until a cursor position report has arrived, the
// deadline has passed or the input has ended. Bytes before the report are
// dropped; a terminal that sends 256 bytes without one is given up on.
func readPosition(t tty, deadline time.Time) (row, col int, ok bool) {
	var got []byte
	buf := make([]byte, 64)
	for len(got) < 256 {
		n, err := t.read(buf, deadline)
		if err != nil {
			return 0, 0, false
		}
		got = append(got, buf[:n]...)
		if row, col, ok = parsePosition(got); ok {
			return row, col, true
		}
	}
	return 0, 0, false
}

// parsePosition finds the first cursor position report in b: ESC [, a row, a
// semicolon, a column, R, with one to five digits in each number and neither
// of them zero.
func parsePosition(b []byte) (row, col int, ok bool) {
	for i := 0; i+1 < len(b); i++ {
		if b[i] != escByte || b[i+1] != '[' {
			continue
		}
		j := i + 2
		row, j = digits(b, j)
		if row == 0 || j >= len(b) || b[j] != ';' {
			continue
		}
		col, j = digits(b, j+1)
		if col == 0 || j >= len(b) || b[j] != 'R' {
			continue
		}
		return row, col, true
	}
	return 0, 0, false
}

// digits reads a decimal number of at most five digits at b[i:]. It returns
// the number and the index after it, or 0 when there is no number or it is
// longer.
func digits(b []byte, i int) (n, next int) {
	start := i
	for i < len(b) && b[i] >= '0' && b[i] <= '9' && i-start < 6 {
		n = n*10 + int(b[i]-'0')
		i++
	}
	if i-start > 5 {
		return 0, i
	}
	return n, i
}
