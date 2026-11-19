package termsafe

import (
	"strings"
	"testing"
)

func TestText_ASCII(t *testing.T) {
	cases := []struct {
		name string
		text Text
		want string
	}{
		{"empty", Text{}, ""},
		{"ascii is returned as it is", Drop.Clean("plain text\n\twith lines"), "plain text\n\twith lines"},
		{"an accented letter", Drop.Line("caf" + u(0x00E9)), "caf<U+00E9>"},
		{"a letter whose second byte is the 8-bit CSI", Drop.Line("a" + u(0x00DB) + "b"), "a<U+00DB>b"},
		{"Han, then ASCII, then an emoji outside the basic plane", Drop.Line(u(0x65E5) + u(0x672C) + " ok " + u(0x1F3AC)), "<U+65E5><U+672C> ok <U+1F3AC>"},
		{"a combining mark is shown by itself", Drop.Line("e" + combAcute), "e<U+0301>"},
		{"an emoji sequence, code point by code point", Drop.Line(heart + vs16 + zwj + u(0x1F525)), "<U+2764><U+FE0F><U+200D><U+1F525>"},
		{"what Mark marked stays marked", Mark.Line("a" + rlo + u(0x00E9)), "a<U+202E><U+00E9>"},
		{"the replacement character of an invalid byte", Drop.Line("na\xefve"), "na<U+FFFD>ve"},
		{"lines are kept", Drop.Clean(u(0x00E9) + "\n\t" + u(0x00E9)), "<U+00E9>\n\t<U+00E9>"},
		{"right-to-left text", Drop.Line(hebrewShalom), "<U+05E9><U+05DC><U+05D5><U+05DD>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.text.ASCII()
			if got.String() != tc.want {
				t.Errorf("ASCII of %+q = %q, want %q", tc.text.String(), got.String(), tc.want)
			}
			if c, bad := firstNotASCII(got.String(), false); bad {
				t.Errorf("ASCII of %+q holds the byte %#x", tc.text.String(), c)
			}
			if again := got.ASCII(); again != got {
				t.Errorf("ASCII twice gives %q, once %q", again.String(), got.String())
			}
		})
	}
}

// TestSamplesThroughASCII: the ASCII form of every hostile sample, cleaned
// either way under either policy, is printable ASCII and nothing else. No
// byte of it can be a control in any terminal whose encoding has ASCII in it.
func TestSamplesThroughASCII(t *testing.T) {
	for _, s := range Samples() {
		for name, policy := range map[string]Policy{"Drop": Drop, "Mark": Mark} {
			line := policy.Line(s.Text).ASCII().String()
			if c, bad := firstNotASCII(line, true); bad {
				t.Errorf("%s.Line, sample %q: the ASCII form %q holds the byte %#x", name, s.Name, line, c)
			}
			clean := policy.Clean(s.Text).ASCII().String()
			if c, bad := firstNotASCII(clean, false); bad {
				t.Errorf("%s.Clean, sample %q: the ASCII form %q holds the byte %#x", name, s.Name, clean, c)
			}
			for _, out := range []string{line, clean} {
				if !strings.Contains(out, SampleMarker) {
					t.Errorf("%s, sample %q: the marker is gone from %q", name, s.Name, out)
				}
				if p := ViewProblem(out); p != "" {
					t.Errorf("%s, sample %q: ViewProblem = %q for %q", name, s.Name, p, out)
				}
				if p := TerminalProblem(out); p != "" {
					t.Errorf("%s, sample %q: TerminalProblem = %q for %q", name, s.Name, p, out)
				}
				if p := ASCIIProblem(out); p != "" {
					t.Errorf("%s, sample %q: ASCIIProblem = %q for %q", name, s.Name, p, out)
				}
			}
		}
	}
}

// TestMixedScriptsBeforeASCII: the ASCII form has no letters of other scripts
// left, so the check has to come first.
func TestMixedScriptsBeforeASCII(t *testing.T) {
	text := Mark.Line("req" + grUpsilon + "ests")
	if n := len(text.MixedScripts()); n != 1 {
		t.Fatalf("%d findings before ASCII, want 1", n)
	}
	if found := text.ASCII().MixedScripts(); found != nil {
		t.Errorf("findings after ASCII: %v", found)
	}
}
