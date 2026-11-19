package termsafe

import (
	"strings"
	"testing"
)

// The kit against the package it tests: cleaned under either policy, every
// sample passes both checks; raw, every sample fails the view check, and the
// ones a renderer would not stop fail the terminal check.
func TestKit_CleanedSamplesPass(t *testing.T) {
	for _, s := range Samples() {
		if strings.ContainsAny(s.Text, " \t\n") {
			t.Errorf("sample %q holds white space", s.Name)
		}
		for name, policy := range map[string]Policy{"Drop": Drop, "Mark": Mark} {
			view := "\x1b[1;38;5;86mtitle\x1b[m\n  \x1b[38;5;241m[log] " + policy.Line(s.Text).String() + "\x1b[m\n"
			if p := ViewProblem(view); p != "" {
				t.Errorf("%s, sample %q: ViewProblem = %q for %+q", name, s.Name, p, view)
			}
			if p := TerminalProblem("\x1b[?1049h\x1b[H" + view + "\x1b[?1049l"); p != "" {
				t.Errorf("%s, sample %q: TerminalProblem = %q", name, s.Name, p)
			}
		}
	}
}

func TestKit_RawSamplesFail(t *testing.T) {
	for _, s := range Samples() {
		view := "\x1b[38;5;241m[log] " + s.Text + "\x1b[m\n"
		if p := ViewProblem(view); p == "" {
			t.Errorf("sample %q: ViewProblem finds nothing wrong with the raw text %+q", s.Name, view)
		}
	}
	// What reaches the terminal when a view is not cleaned depends on the
	// renderer; the check itself must see each of these.
	for _, s := range Samples() {
		switch s.Name {
		case "csi erase and home", "joiner between Latin letters", "variation selector after a Latin letter":
			// A renderer's own CSI cannot be told apart. A joiner or a
			// single selector is right or wrong by what it follows,
			// which is judged in the view.
			continue
		}
		if p := TerminalProblem(s.Text); p == "" {
			t.Errorf("sample %q: TerminalProblem finds nothing wrong with the raw text", s.Name)
		}
	}
}

func TestKit_TerminalProblems(t *testing.T) {
	emoji := SampleMarker + heart + vs16 // an emoji with its selector
	for written, want := range map[string]string{
		SampleMarker + " plain": "",
		emoji:                   "",
		SampleMarker + lineSep:  "line break U+2028",
		SampleMarker + paraSep:  "line break U+2029",
		emoji + vs16:            "run of variation selectors",
	} {
		got := TerminalProblem("\x1b[?1049h" + written + "\x1b[?1049l")
		if (want == "") != (got == "") || !strings.Contains(got, want) {
			t.Errorf("TerminalProblem(%+q) = %q, want it to mention %q", written, got, want)
		}
	}
}

func TestKit_NothingDrawn(t *testing.T) {
	if p := ViewProblem("an empty screen"); !strings.Contains(p, "not drawn") {
		t.Errorf("ViewProblem = %q, want it to say the text is not drawn", p)
	}
	if p := TerminalProblem("\x1b[?1049h"); !strings.Contains(p, "never drawn") {
		t.Errorf("TerminalProblem = %q, want it to say the text was never drawn", p)
	}
}

func TestKit_ASCIIProblem(t *testing.T) {
	ownSequences := "\x1b[1;38;5;86mtitle\x1b[m\x1b[2K\r" // a renderer's own, all ASCII
	for written, want := range map[string]string{
		"":                                "",
		SampleMarker + " plain\n\ttext":   "",
		ownSequences:                      "",
		"caf" + u(0x00E9):                 "U+00E9",
		"a bullet " + u(0x2022):           "U+2022",
		"a raw byte \x9b":                 "the byte 0x9B",
		"half a character \xc3":           "the byte 0xC3",
		"a replacement char " + u(0xFFFD): "U+FFFD",
	} {
		got := ASCIIProblem(written)
		if (want == "") != (got == "") || !strings.Contains(got, want) {
			t.Errorf("ASCIIProblem(%+q) = %q, want it to mention %q", written, got, want)
		}
	}
}

func TestKit_ViewProblems(t *testing.T) {
	// An emoji with its selector, which is legitimate, and the same with a
	// second selector after it.
	emoji := SampleMarker + heart + vs16
	for view, want := range map[string]string{
		SampleMarker + " plain":                 "",
		SampleMarker + "\x1b[1mbold\x1b[0m":     "",
		SampleMarker + "\xffnext":               "1 invalid byte",
		SampleMarker + " +q" + samplePayload:    "payload",
		SampleMarker + "\x1b[38;5;201mcoloured": "colour sequence",
		SampleMarker + "\x1b[2J":                "control character U+001B",
		SampleMarker + "\rnext":                 "control character U+000D",
		SampleMarker + rlo:                      "bidirectional control U+202E",
		SampleMarker + zwsp:                     "invisible character U+200B",
		SampleMarker + "a" + zwj:                "1 invisible character",
		SampleMarker + lineSep:                  "line break U+2028",
		SampleMarker + paraSep:                  "line break U+2029",
		SampleMarker + vs16:                     "1 invisible character",
		emoji:                                   "",
		emoji + vs17:                            "run of variation selectors, U+E0100 after U+FE0F",
	} {
		got := ViewProblem(view)
		if (want == "") != (got == "") || !strings.Contains(got, want) {
			t.Errorf("ViewProblem(%+q) = %q, want it to mention %q", view, got, want)
		}
	}
}
