package termsafe

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// emulator is a terminal as far as the probe can tell one from another: a
// screen with a cursor, a way of reading the bytes of a character, and
// answers to the cursor position query.
type emulator struct {
	decodesUTF8 bool // one cell for a UTF-8 character
	doubleByte  bool // an East Asian double-byte set: two high bytes are one wide character
	row, col    int
	drawn       map[[2]int]bool // the cells that hold something
	queries     int
	// answer, when set, replaces the true report for the n-th query. It may
	// return nothing.
	answer func(n int, report string) string
}

func newEmulator(decodesUTF8 bool) *emulator {
	return &emulator{decodesUTF8: decodesUTF8, row: 7, col: 12, drawn: map[[2]int]bool{}}
}

// feed takes what the program wrote and returns what the terminal sends
// back.
func (e *emulator) feed(b []byte) (reply []byte) {
	for len(b) > 0 {
		var row, col int
		switch {
		case strings.HasPrefix(string(b), cursorQuery):
			e.queries++
			report := fmt.Sprintf("\x1b[%d;%dR", e.row, e.col)
			if e.answer != nil {
				report = e.answer(e.queries, report)
			}
			reply = append(reply, report...)
			b = b[len(cursorQuery):]
		case scanPosition(b, &row, &col) > 0:
			b = b[scanPosition(b, &row, &col):]
			e.row, e.col = row, col
		case b[0] == ' ':
			delete(e.drawn, [2]int{e.row, e.col})
			e.col++
			b = b[1:]
		case b[0] < utf8.RuneSelf:
			e.draw(1)
			b = b[1:]
		case e.decodesUTF8:
			_, size := utf8.DecodeRune(b)
			e.draw(1)
			b = b[size:]
		case e.doubleByte && len(b) >= 2:
			e.draw(2)
			b = b[2:]
		default:
			e.draw(1)
			b = b[1:]
		}
	}
	return reply
}

func (e *emulator) draw(cells int) {
	for ; cells > 0; cells-- {
		e.drawn[[2]int{e.row, e.col}] = true
		e.col++
	}
}

// scanPosition reads ESC [ row ; col H at the start of b and returns its
// length, or 0.
func scanPosition(b []byte, row, col *int) int {
	var end byte
	if n, _ := fmt.Sscanf(string(b), "\x1b[%d;%d%c", row, col, &end); n != 3 || end != 'H' {
		return 0
	}
	return strings.IndexByte(string(b), 'H') + 1
}

// untouched reports whether the screen is as it was before the probe:
// nothing drawn and the cursor where it stood.
func (e *emulator) untouched() bool {
	return len(e.drawn) == 0 && e.row == 7 && e.col == 12
}

// scripted is an emulator behind the tty interface, with no real time: an
// answer either is there before the deadline or is not.
type scripted struct {
	*emulator
	answersAt   time.Time // when its answers arrive; the zero time is at once
	inbox       []byte
	chunk       int // at most this many bytes to a read, 0 for all
	failWrite   int // the write with this number fails, 0 for none
	writes      int
	written     []byte
	readsFailed error // what read returns when the inbox is empty
}

func (s *scripted) write(b []byte, deadline time.Time) error {
	s.writes++
	if s.writes == s.failWrite {
		return errDeadline
	}
	s.written = append(s.written, b...)
	s.inbox = append(s.inbox, s.feed(b)...)
	return nil
}

func (s *scripted) read(buf []byte, deadline time.Time) (int, error) {
	if len(s.inbox) == 0 || s.answersAt.After(deadline) {
		if s.readsFailed != nil {
			return 0, s.readsFailed
		}
		return 0, errDeadline
	}
	n := len(s.inbox)
	if s.chunk > 0 && n > s.chunk {
		n = s.chunk
	}
	n = copy(buf, s.inbox[:n])
	s.inbox = s.inbox[n:]
	return n, nil
}

// The scripted terminals of TestAsk.
func utf8Terminal() *scripted     { return &scripted{emulator: newEmulator(true)} }
func eightBitTerminal() *scripted { return &scripted{emulator: newEmulator(false)} }

func TestAsk(t *testing.T) {
	const asked = "the terminal was asked: it "
	deadline := time.Now().Add(time.Second)

	silentAfter := func(first int) func(int, string) string {
		return func(n int, report string) string {
			if n > first {
				return ""
			}
			return report
		}
	}

	cases := []struct {
		name        string
		terminal    *scripted
		columns     int
		want        Encoding
		wantFinding string
		wantWritten string // when set, exactly what the terminal was sent
	}{
		{
			name:        "a terminal that decodes UTF-8",
			terminal:    utf8Terminal(),
			want:        EncodingUTF8,
			wantFinding: asked + "drew a character of two UTF-8 bytes in one cell, as a terminal that decodes UTF-8 does",
			wantWritten: cursorQuery + probeCharacter + cursorQuery + "\x1b[7;12H \x1b[7;12H",
		},
		{
			name:        "an 8-bit terminal: two characters",
			terminal:    eightBitTerminal(),
			want:        EncodingNotUTF8,
			wantFinding: asked + "drew a character of two UTF-8 bytes in two cells, as a terminal that does not decode UTF-8 does",
			wantWritten: cursorQuery + probeCharacter + cursorQuery + "\x1b[7;12H  \x1b[7;12H",
		},
		{
			name: "an East Asian double-byte terminal: one wide character",
			terminal: func() *scripted {
				s := eightBitTerminal()
				s.doubleByte = true
				return s
			}(),
			want:        EncodingNotUTF8,
			wantFinding: asked + "drew a character of two UTF-8 bytes in two cells, as a terminal that does not decode UTF-8 does",
		},
		{
			name: "a terminal that never answers: nothing is drawn",
			terminal: func() *scripted {
				s := utf8Terminal()
				s.answer = silentAfter(0)
				return s
			}(),
			want:        EncodingUnknown,
			wantFinding: "the terminal was asked for the cursor position and did not answer in time",
			wantWritten: cursorQuery,
		},
		{
			name: "a terminal that answers with garbage: nothing is drawn",
			terminal: func() *scripted {
				s := utf8Terminal()
				s.answer = func(int, string) string { return strings.Repeat("\x1b[?1;2c\x00garbage\xff", 40) }
				return s
			}(),
			want:        EncodingUnknown,
			wantFinding: "the terminal was asked for the cursor position and did not answer in time",
			wantWritten: cursorQuery,
		},
		{
			name: "a terminal that answers after the deadline: nothing is drawn",
			terminal: func() *scripted {
				s := utf8Terminal()
				s.answersAt = deadline.Add(time.Millisecond)
				return s
			}(),
			want:        EncodingUnknown,
			wantFinding: "the terminal was asked for the cursor position and did not answer in time",
			wantWritten: cursorQuery,
		},
		{
			name: "a terminal that answers once: what was drawn is blanked",
			terminal: func() *scripted {
				s := eightBitTerminal()
				s.answer = silentAfter(1)
				return s
			}(),
			want:        EncodingUnknown,
			wantFinding: asked + "answered the first cursor position query and not the second in time",
			wantWritten: cursorQuery + probeCharacter + cursorQuery + "\x1b[7;12H  \x1b[7;12H",
		},
		{
			name: "the input ends",
			terminal: func() *scripted {
				s := utf8Terminal()
				s.answer = silentAfter(0)
				s.readsFailed = errors.New("EOF")
				return s
			}(),
			want:        EncodingUnknown,
			wantFinding: "the terminal was asked for the cursor position and did not answer in time",
		},
		{
			name:        "the cursor within two cells of the right edge: nothing is drawn",
			terminal:    utf8Terminal(),
			columns:     13,
			want:        EncodingUnknown,
			wantFinding: "the terminal was not asked: the cursor is within two cells of the right edge",
			wantWritten: cursorQuery,
		},
		{
			name:        "two cells from the right edge is enough",
			terminal:    eightBitTerminal(),
			columns:     14,
			want:        EncodingNotUTF8,
			wantFinding: asked + "drew a character of two UTF-8 bytes in two cells, as a terminal that does not decode UTF-8 does",
		},
		{
			name: "keys pressed early stand before the answer, which arrives a byte at a time",
			terminal: func() *scripted {
				s := utf8Terminal()
				s.answer = func(n int, report string) string { return "q\x1b[A\x1b[" + report }
				s.chunk = 1
				return s
			}(),
			want:        EncodingUTF8,
			wantFinding: asked + "drew a character of two UTF-8 bytes in one cell, as a terminal that decodes UTF-8 does",
		},
		{
			name: "the second answer names another row",
			terminal: func() *scripted {
				s := utf8Terminal()
				s.answer = func(n int, report string) string {
					if n == 2 {
						return "\x1b[8;1R"
					}
					return report
				}
				return s
			}(),
			want:        EncodingUnknown,
			wantFinding: asked + "moved the cursor from row 7 column 12 to row 8 column 1, which tells nothing",
			wantWritten: cursorQuery + probeCharacter + cursorQuery + "\x1b[7;12H  \x1b[7;12H",
		},
		{
			name: "the cursor did not move",
			terminal: func() *scripted {
				s := utf8Terminal()
				s.answer = func(int, string) string { return "\x1b[7;12R" }
				return s
			}(),
			want:        EncodingUnknown,
			wantFinding: asked + "moved the cursor from row 7 column 12 to row 7 column 12, which tells nothing",
		},
		{
			name: "the terminal cannot be written to at all",
			terminal: func() *scripted {
				s := utf8Terminal()
				s.failWrite = 1
				return s
			}(),
			want:        EncodingUnknown,
			wantFinding: "the terminal was asked and could not be written to in time",
			wantWritten: "",
		},
		{
			name: "the test character cannot be written: the line is put back all the same",
			terminal: func() *scripted {
				s := utf8Terminal()
				s.failWrite = 2
				return s
			}(),
			want:        EncodingUnknown,
			wantFinding: asked + "answered once and could then not be written to in time",
			wantWritten: cursorQuery + "\x1b[7;12H  \x1b[7;12H",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			columns := tc.columns
			if columns == 0 {
				columns = 80
			}
			got, finding := ask(tc.terminal, columns, deadline)
			if got != tc.want {
				t.Errorf("verdict %v, want %v", got, tc.want)
			}
			if finding != tc.wantFinding {
				t.Errorf("finding %q\n   want %q", finding, tc.wantFinding)
			}
			if tc.wantWritten != "" || tc.terminal.failWrite == 1 {
				if string(tc.terminal.written) != tc.wantWritten {
					t.Errorf("the terminal was sent %q\n                 want %q", tc.terminal.written, tc.wantWritten)
				}
			}
			if !tc.terminal.untouched() {
				t.Errorf("the screen is not as it was: cells %v, cursor at %d;%d", tc.terminal.drawn, tc.terminal.row, tc.terminal.col)
			}
			if b, bad := firstNotASCII(finding, true); bad {
				t.Errorf("the finding %q holds the byte %#x", finding, b)
			}
		})
	}
}

// TestAsk_CannotErase: the one case that leaves something on the screen says
// so and claims nothing else.
func TestAsk_CannotErase(t *testing.T) {
	s := utf8Terminal()
	s.failWrite = 3
	got, finding := ask(s, 80, time.Now().Add(time.Second))
	if got != EncodingUnknown || !strings.Contains(finding, "the character may still be drawn") {
		t.Errorf("verdict %v, finding %q", got, finding)
	}
	if s.untouched() {
		t.Error("the emulator says nothing was left, so this test tests nothing")
	}
}

func TestParsePosition(t *testing.T) {
	cases := []struct {
		in       string
		row, col int
		ok       bool
	}{
		{"\x1b[7;12R", 7, 12, true},
		{"\x1b[1;1R", 1, 1, true},
		{"\x1b[99999;99999R", 99999, 99999, true},
		{"typed\x1b[A\x1b[24;80Rmore", 24, 80, true},
		{"\x1b[7;\x1b[3;4R", 3, 4, true}, // a broken report, then a whole one
		{"\x1b[7;12R\x1b[9;9R", 7, 12, true},
		{"", 0, 0, false},
		{"\x1b", 0, 0, false},
		{"\x1b[", 0, 0, false},
		{"\x1b[7", 0, 0, false},
		{"\x1b[7;", 0, 0, false},
		{"\x1b[7;12", 0, 0, false},
		{"\x1b[7;12H", 0, 0, false},
		{"\x1b[;12R", 0, 0, false},
		{"\x1b[7;R", 0, 0, false},
		{"\x1b[0;12R", 0, 0, false},
		{"\x1b[7;0R", 0, 0, false},
		{"\x1b[123456;1R", 0, 0, false},
		{"\x1b[1;123456R", 0, 0, false},
		{"\x1b[?1;2c", 0, 0, false}, // a device attributes report
		{"7;12R", 0, 0, false},
	}
	for _, tc := range cases {
		row, col, ok := parsePosition([]byte(tc.in))
		if ok != tc.ok || (ok && (row != tc.row || col != tc.col)) {
			t.Errorf("parsePosition(%q) = %d, %d, %v; want %d, %d, %v", tc.in, row, col, ok, tc.row, tc.col, tc.ok)
		}
	}
}

func TestRefusal(t *testing.T) {
	terminal := ttyState{inTTY: true, outTTY: true, raw: true, columns: 80}
	change := func(f func(*ttyState)) ttyState {
		st := terminal
		f(&st)
		return st
	}
	cases := []struct {
		name      string
		haveFiles bool
		st        ttyState
		want      string
	}{
		{"a terminal in raw mode", true, terminal, ""},
		{"no files", false, terminal, "it was not given an open input and output"},
		{"the input is a pipe", true, change(func(st *ttyState) { st.inTTY = false }), "the input and the output are not both a terminal"},
		{"the output is a pipe", true, change(func(st *ttyState) { st.outTTY = false }), "the input and the output are not both a terminal"},
		{"line editing or echo is on", true, change(func(st *ttyState) { st.raw = false }), "the terminal is not in raw mode, so it would draw the answer and hold it back until a newline"},
		{"no width", true, change(func(st *ttyState) { st.columns = 0 }), "the width of the terminal is not known"},
		{"three columns", true, change(func(st *ttyState) { st.columns = 3 }), "the width of the terminal is not known"},
		{"nothing at all", false, ttyState{}, "it was not given an open input and output"},
	}
	for _, tc := range cases {
		if got := refusal(tc.haveFiles, tc.st); got != tc.want {
			t.Errorf("%s: refusal = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestProbeWith: what an answer, and the lack of one, do to a Terminal.
func TestProbeWith(t *testing.T) {
	saysUTF8 := judge(outputFacts{kind: outTerminal}, env{"LANG": "en_US.UTF-8"}.get)
	saysNot := judge(outputFacts{kind: outTerminal}, env{"LANG": "C"}.get)
	cannotTell := judge(outputFacts{kind: outTerminal}, env{}.get)

	cases := []struct {
		name       string
		before     Terminal
		terminal   *scripted
		refusal    string
		timeout    time.Duration
		want       Encoding
		wantReason string // "" for the reason the environment gave
		wantLast   string // the start of the evidence line the probe added
		wantWrites int
	}{
		{
			name:       "the environment says UTF-8 and the terminal shows it is not: the terminal wins",
			before:     saysUTF8,
			terminal:   eightBitTerminal(),
			timeout:    time.Second,
			want:       EncodingNotUTF8,
			wantReason: "the terminal was asked: it drew a character of two UTF-8 bytes in two cells, as a terminal that does not decode UTF-8 does (the environment said: UTF-8)",
			wantLast:   "the terminal was asked: it drew a character of two UTF-8 bytes in two cells",
			wantWrites: 3,
		},
		{
			name:       "the environment says the C locale and the terminal shows UTF-8: the terminal wins",
			before:     saysNot,
			terminal:   utf8Terminal(),
			timeout:    time.Second,
			want:       EncodingUTF8,
			wantReason: "the terminal was asked: it drew a character of two UTF-8 bytes in one cell, as a terminal that decodes UTF-8 does (the environment said: not UTF-8)",
			wantLast:   "the terminal was asked: it drew a character of two UTF-8 bytes in one cell",
			wantWrites: 3,
		},
		{
			name:       "the environment says nothing and the terminal shows UTF-8",
			before:     cannotTell,
			terminal:   utf8Terminal(),
			timeout:    time.Second,
			want:       EncodingUTF8,
			wantReason: "the terminal was asked: it drew a character of two UTF-8 bytes in one cell, as a terminal that decodes UTF-8 does (the environment said: cannot tell)",
			wantLast:   "the terminal was asked: it drew",
			wantWrites: 3,
		},
		{
			name:   "no answer: the environment's verdict stands",
			before: saysUTF8,
			terminal: func() *scripted {
				s := utf8Terminal()
				s.answer = func(int, string) string { return "" }
				return s
			}(),
			timeout:    time.Second,
			want:       EncodingUTF8,
			wantLast:   "the terminal was asked for the cursor position and did not answer in time",
			wantWrites: 1,
		},
		{
			name:     "not a terminal: nothing is written",
			before:   saysNot,
			terminal: utf8Terminal(),
			refusal:  "the input and the output are not both a terminal",
			timeout:  time.Second,
			want:     EncodingNotUTF8,
			wantLast: "the terminal was not asked: the input and the output are not both a terminal",
		},
		{
			name:     "no time allowed: nothing is written",
			before:   saysUTF8,
			terminal: utf8Terminal(),
			timeout:  0,
			want:     EncodingUTF8,
			wantLast: "the terminal was not asked: no time was allowed for an answer",
		},
		{
			name:     "a negative timeout",
			before:   cannotTell,
			terminal: utf8Terminal(),
			timeout:  -time.Second,
			want:     EncodingUnknown,
			wantLast: "the terminal was not asked: no time was allowed for an answer",
		},
		{
			name:       "a Terminal nobody filled in can be probed too",
			before:     Terminal{},
			terminal:   utf8Terminal(),
			timeout:    time.Second,
			want:       EncodingUTF8,
			wantReason: "the terminal was asked: it drew a character of two UTF-8 bytes in one cell, as a terminal that decodes UTF-8 does (the environment said: cannot tell)",
			wantLast:   "the terminal was asked: it drew",
			wantWrites: 3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evidenceBefore := evidenceLines(tc.before)
			got := tc.before.probeWith(tc.terminal, 80, tc.refusal, tc.timeout)

			if got.Encoding != tc.want {
				t.Errorf("Encoding = %v, want %v", got.Encoding, tc.want)
			}
			wantReason := tc.wantReason
			if wantReason == "" {
				wantReason = tc.before.Reason.String()
			}
			if got.Reason.String() != wantReason {
				t.Errorf("Reason = %q\n  want %q", got.Reason.String(), wantReason)
			}
			// The evidence records both: every line of the environment,
			// then one for the probe.
			if len(got.Evidence) != len(tc.before.Evidence)+1 {
				t.Fatalf("%d lines of evidence, want %d and one more", len(got.Evidence), len(tc.before.Evidence))
			}
			if lines := evidenceLines(got); !strings.HasPrefix(lines, evidenceBefore) {
				t.Errorf("the evidence of the environment is gone:\n%s", lines)
			}
			if last := got.Evidence[len(got.Evidence)-1].String(); !strings.HasPrefix(last, tc.wantLast) {
				t.Errorf("the probe's line is %q, want it to start %q", last, tc.wantLast)
			}
			if evidenceLines(tc.before) != evidenceBefore {
				t.Error("Probe changed the Terminal it was called on")
			}
			if tc.terminal.writes != tc.wantWrites {
				t.Errorf("%d writes to the terminal, want %d", tc.terminal.writes, tc.wantWrites)
			}
			if !tc.terminal.untouched() {
				t.Error("the screen is not as it was")
			}
			for _, line := range append([]Text{got.Reason}, got.Evidence[len(got.Evidence)-1]) {
				if line.IsEmpty() {
					continue // a Terminal nobody filled in has no reason
				}
				if b, bad := firstNotASCII(line.String(), true); bad {
					t.Errorf("%q holds the byte %#x", line.String(), b)
				}
			}
			// Fit follows the new verdict.
			text := Drop.Line("caf" + u(0x00E9))
			if fitted := got.Fit(text); (fitted == text) != (tc.want == EncodingUTF8) {
				t.Errorf("Fit after the probe gives %+q with the verdict %v", fitted.String(), tc.want)
			}
		})
	}
}

// TestProbe_Evidence_NotShared: two probes of one Terminal do not write into
// each other's evidence.
func TestProbe_EvidenceNotShared(t *testing.T) {
	base := judge(outputFacts{kind: outTerminal}, env{"LANG": "en_US.UTF-8"}.get)
	base.Evidence = append(make([]Text, 0, 16), base.Evidence...) // room to grow in place
	first := base.probeWith(utf8Terminal(), 80, "", time.Second)
	second := base.probeWith(eightBitTerminal(), 80, "", time.Second)
	if first.Encoding != EncodingUTF8 || second.Encoding != EncodingNotUTF8 {
		t.Fatalf("verdicts %v and %v", first.Encoding, second.Encoding)
	}
	if !strings.Contains(first.Evidence[len(first.Evidence)-1].String(), "in one cell") {
		t.Errorf("the second probe wrote over the first one's evidence: %q", first.Evidence[len(first.Evidence)-1])
	}
}

// TestProbeCharacter pins what the probe draws: U+00E4, whose two bytes are
// both above 0x9F, so that neither is a C1 control to an 8-bit terminal.
func TestProbeCharacter(t *testing.T) {
	r, size := utf8.DecodeRuneInString(probeCharacter)
	if r != 0x00E4 || size != len(probeCharacter) {
		t.Fatalf("the probe character is %U in %d of %d bytes", r, size, len(probeCharacter))
	}
	for i := 0; i < len(probeCharacter); i++ {
		if c := probeCharacter[i]; c < 0xA0 {
			t.Errorf("byte %d of the probe character is %#x, a control or ASCII to an 8-bit terminal", i, c)
		}
	}
	for i := 0; i < len(cursorQuery); i++ {
		if cursorQuery[i] >= utf8.RuneSelf {
			t.Errorf("the cursor query holds the byte %#x", cursorQuery[i])
		}
	}
}
