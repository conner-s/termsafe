package termsafe

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file is the test kit: hostile samples, and the two checks a program
// runs to prove that none of them gets through it. It is in the package, not
// in one beside it: the checks use the cleaner's own tables, and a program's
// tests get the samples that belong to the version of the cleaner it imports.
// PORTING.md says how to write the two tests.

// A Sample is one piece of hostile outside text.
type Sample struct {
	Name string
	Text string
}

// SampleMarker is a word every sample holds. A test looks for it to know
// that the sample was drawn at all, so that it cannot pass by drawing
// nothing.
const SampleMarker = "VISIBLE"

// samplePayload stands inside the sequences of the samples. Whole sequences
// are removed, payload and all, so it must never be drawn, not even as text.
const samplePayload = "544e"

// Samples returns the hostile samples: control sequences in their ESC and C1
// forms, terminated and not, single controls, raw 8-bit bytes, bidirectional
// controls, invisible characters, variation selectors where they select
// nothing, and line breaks that are not a newline. Each holds [SampleMarker];
// none holds a space, a tab or a newline, so each also fits where a tool
// prints one word, such as a size or a version.
//
// The list grows: a release that changes what cleaning does adds the sample
// that shows it.
func Samples() []Sample {
	const (
		esc = "\x1b"
		st  = esc + `\`
		bel = "\x07"
	)
	u := func(r rune) string { return string(r) }
	return []Sample{
		{"dcs", SampleMarker + esc + "P+q" + samplePayload + st + "tail"},
		{"dcs unterminated", SampleMarker + esc + "P+q" + samplePayload},
		{"apc", SampleMarker + esc + "_Gf=100;" + samplePayload + st + "tail"},
		{"apc unterminated", SampleMarker + esc + "_Gf=100;" + samplePayload},
		{"pm", SampleMarker + esc + "^" + samplePayload + st + "tail"},
		{"sos", SampleMarker + esc + "X" + samplePayload + st + "tail"},
		{"osc 52", SampleMarker + esc + "]52;c;" + samplePayload + bel + "tail"},
		{"osc 8", esc + "]8;;https://" + samplePayload + ".example/" + st + SampleMarker + esc + "]8;;" + st},
		{"osc title", SampleMarker + esc + "]0;" + samplePayload + bel + "tail"},
		{"csi erase and home", SampleMarker + esc + "[2J" + esc + "[H" + esc + "[38;5;201mtail"},
		{"ris", SampleMarker + esc + "ctail"},
		{"c1 dcs", SampleMarker + u(0x90) + "+q" + samplePayload + u(0x9c) + "tail"},
		{"c1 csi", SampleMarker + u(0x9b) + "2Jtail"},
		{"c1 osc 52", SampleMarker + u(0x9d) + "52;c;" + samplePayload + u(0x9c) + "tail"},
		{"raw 8-bit dcs", SampleMarker + "\x90+q\x9ctail"},
		{"c0", SampleMarker + "\x07\x08\x00\x0e\x7ftail"},
		{"utf8", u(0x65e5) + u(0x672c) + SampleMarker + esc + "P+q" + samplePayload + st + u(0x1f3ac)},
		{"right-to-left override", SampleMarker + u(0x202e) + "tail"},
		{"isolate unterminated", u(0x2067) + SampleMarker},
		{"left-to-right mark", SampleMarker + u(0x200e) + "tail"},
		{"zero width space", SampleMarker + u(0x200b) + "tail"},
		{"joiner between Latin letters", SampleMarker + u(0x200d) + "tail"},
		{"byte order mark", u(0xfeff) + SampleMarker},
		{"tag characters", SampleMarker + u(0xe0078) + u(0xe0079)},
		{"variation selector run", SampleMarker + u(0xfe00) + u(0xe0100) + u(0xe0155) + "tail"},
		{"variation selector after a Latin letter", SampleMarker + u(emojiSelector) + "tail"},
		{"second variation selector", u(0x2764) + u(emojiSelector) + u(emojiSelector) + SampleMarker},
		{"line and paragraph separators", SampleMarker + u(lineSeparator) + "mid" + u(paragraphSeparator) + "tail"},
	}
}

// ViewProblem examines the string a view hands to its TUI framework after a
// sample was fed in, and says what is wrong with it; "" means nothing is.
//
// The view may hold SGR sequences, the colours and attributes a program
// styles its own text with. Those are set aside. In what is left there must
// be the marker, and nothing that cleaning would still remove, mark or
// replace: no control sequence, control character, invalid byte,
// bidirectional control, invisible character, run of variation selectors or
// line separator, and no payload of a sample's sequence drawn as text.
func ViewProblem(view string) string {
	if !strings.Contains(view, SampleMarker) {
		return "the text is not drawn at all (no " + SampleMarker + ")"
	}
	if strings.Contains(view, "\x1b[38;5;201m") {
		return "the outside text's own colour sequence is in the view"
	}
	plain := withoutSGR(view)
	// Character by character first, without the cleaner, so that this
	// check does not lean on the code it is there to check.
	var before rune
	for _, r := range plain {
		switch {
		case r == '\n' || r == '\t':
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			return fmt.Sprintf("the view holds the control character U+%04X", r)
		case isBidiControl(r):
			return fmt.Sprintf("the view holds the bidirectional control U+%04X", r)
		case r == 0x200b || r == 0xfeff || (r >= tagFirst && r <= tagLast):
			return fmt.Sprintf("the view holds the invisible character U+%04X", r)
		case r == lineSeparator || r == paragraphSeparator:
			return fmt.Sprintf("the view holds the line break U+%04X", r)
		case isSelectorRun(before, r):
			return fmt.Sprintf("the view holds a run of variation selectors, U+%04X after U+%04X", r, before)
		}
		before = r
	}
	// Then everything the cleaner knows, which is more: invalid bytes,
	// every invisible character, a joiner where it can do nothing.
	if _, rep := Mark.CleanReport(plain); rep.Changed() {
		return "the view still holds " + rep.String()
	}
	if strings.Contains(plain, samplePayload) {
		return "the payload of a control sequence is drawn as text"
	}
	return ""
}

// TerminalProblem examines every byte a program wrote to its terminal after
// a sample was fed in, and says what is wrong; "" means nothing is.
//
// A renderer writes CSI sequences of its own, so those cannot be judged here;
// [ViewProblem] covers them. What a renderer has no reason to write is looked
// for: the introducers of DCS, APC, PM and SOS strings, an OSC that sets the
// clipboard, a hyperlink or the title, a reset, BEL, any C1 control, any
// bidirectional control, the invisible characters of the samples, two
// variation selectors in a row, a line or paragraph separator, and the
// samples' payload. Run the program with its input disabled, so that it sends
// the terminal no queries of its own.
func TerminalProblem(written string) string {
	if !strings.Contains(written, SampleMarker) {
		return "the text was never drawn (no " + SampleMarker + ")"
	}
	for _, f := range []struct{ what, seq string }{
		{"a DCS introducer", "\x1bP"},
		{"an APC introducer", "\x1b_"},
		{"a PM introducer", "\x1b^"},
		{"an SOS introducer", "\x1bX"},
		{"an OSC 52 clipboard write", "\x1b]52"},
		{"an OSC 8 hyperlink", "\x1b]8;"},
		{"an OSC 0 window title", "\x1b]0;"},
		{"a RIS reset", "\x1bc"},
		{"BEL", "\x07"},
		{"a raw 8-bit DCS", "\x90+q"},
		{"the outside text's own colour sequence", "\x1b[38;5;201m"},
		{"the payload of a control sequence", samplePayload},
	} {
		if strings.Contains(written, f.seq) {
			return f.what + " reached the terminal"
		}
	}
	var before rune
	for _, r := range written {
		switch {
		case r >= 0x80 && r <= 0x9f:
			return fmt.Sprintf("the C1 control U+%04X reached the terminal", r)
		case isBidiControl(r):
			return fmt.Sprintf("the bidirectional control U+%04X reached the terminal", r)
		case r == 0x200b || r == 0xfeff || (r >= tagFirst && r <= tagLast):
			return fmt.Sprintf("the invisible character U+%04X reached the terminal", r)
		case r == lineSeparator || r == paragraphSeparator:
			return fmt.Sprintf("the line break U+%04X reached the terminal", r)
		case isSelectorRun(before, r):
			return fmt.Sprintf("a run of variation selectors reached the terminal, U+%04X after U+%04X", r, before)
		}
		before = r
	}
	return ""
}

// ASCIIProblem examines a view, or every byte a program wrote to its
// terminal, in a run where the program was told that the terminal is not
// UTF-8, and says what is wrong; "" means nothing is.
//
// Nothing outside ASCII may be there: not the outside text, which should have
// gone through [Text.ASCII] or [Terminal.Fit], and not the program's own
// borders, bullets and spinners either, whose UTF-8 bytes an 8-bit terminal
// reads the same way. Use it beside [ViewProblem] and [TerminalProblem], which
// judge the control sequences.
func ASCIIProblem(written string) string {
	for i := 0; i < len(written); i++ {
		if written[i] < utf8.RuneSelf {
			continue
		}
		r, size := utf8.DecodeRuneInString(written[i:])
		if r == utf8.RuneError && size == 1 {
			return fmt.Sprintf("the byte 0x%02X, which is not ASCII, was written for a terminal that is not UTF-8", written[i])
		}
		return fmt.Sprintf("U+%04X, which is not ASCII, was written for a terminal that is not UTF-8", r)
	}
	return ""
}

// isSelectorRun reports whether r is a variation selector that follows
// another. No sequence Unicode defines has two.
func isSelectorRun(before, r rune) bool {
	return unicode.Is(unicode.Variation_Selector, r) && unicode.Is(unicode.Variation_Selector, before)
}

// withoutSGR returns s without its SGR sequences: ESC [, digits, semicolons
// and colons, m.
func withoutSGR(s string) string {
	if !strings.Contains(s, "\x1b[") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == escByte && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == ';' || s[j] == ':') {
				j++
			}
			if j < len(s) && s[j] == 'm' {
				i = j + 1
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		b.WriteString(s[i : i+size])
		i += size
	}
	return b.String()
}
