package termsafe

import (
	"strings"
	"testing"
)

// env is an environment for judge.
type env map[string]string

func (e env) get(name string) string { return e[name] }

func TestJudge_Locale(t *testing.T) {
	cases := []struct {
		name       string
		env        env
		want       Encoding
		wantReason string
	}{
		{"LANG names UTF-8", env{"LANG": "en_US.UTF-8"}, EncodingUTF8, "LANG=en_US.UTF-8 names UTF-8"},
		{"lower case, no hyphen", env{"LANG": "de_DE.utf8"}, EncodingUTF8, "LANG=de_DE.utf8 names UTF-8"},
		{"with a modifier", env{"LANG": "ca_ES.UTF-8@valencia"}, EncodingUTF8, "LANG=ca_ES.UTF-8@valencia names UTF-8"},
		{"the C locale with UTF-8", env{"LC_ALL": "C.UTF-8"}, EncodingUTF8, "LC_ALL=C.UTF-8 names UTF-8"},
		{"macOS sets a bare codeset", env{"LC_CTYPE": "UTF-8"}, EncodingUTF8, "LC_CTYPE=UTF-8 names UTF-8"},

		{"LC_ALL wins over LC_CTYPE and LANG", env{"LC_ALL": "C", "LC_CTYPE": "en_US.UTF-8", "LANG": "en_US.UTF-8"}, EncodingNotUTF8, "LC_ALL=C is the C locale, whose characters are ASCII"},
		{"LC_CTYPE wins over LANG", env{"LC_CTYPE": "ja_JP.eucJP", "LANG": "en_US.UTF-8"}, EncodingNotUTF8, "LC_CTYPE=ja_JP.eucJP names the codeset eucJP, not UTF-8"},
		{"an empty LC_ALL counts as unset", env{"LC_ALL": "", "LC_CTYPE": "", "LANG": "fr_FR.UTF-8"}, EncodingUTF8, "LANG=fr_FR.UTF-8 names UTF-8"},
		{"only LC_CTYPE decides, not LC_MESSAGES", env{"LC_MESSAGES": "en_US.UTF-8", "LANG": "C"}, EncodingNotUTF8, "LANG=C is the C locale, whose characters are ASCII"},

		{"Latin-1", env{"LANG": "en_US.ISO-8859-1"}, EncodingNotUTF8, "LANG=en_US.ISO-8859-1 names the codeset ISO-8859-1, not UTF-8"},
		{"Latin-9 with a modifier", env{"LANG": "de_DE.ISO-8859-15@euro"}, EncodingNotUTF8, "LANG=de_DE.ISO-8859-15@euro names the codeset ISO-8859-15, not UTF-8"},
		{"GB 18030", env{"LANG": "zh_CN.GB18030"}, EncodingNotUTF8, "LANG=zh_CN.GB18030 names the codeset GB18030, not UTF-8"},
		{"UTF-16 is not UTF-8", env{"LANG": "en_US.UTF-16"}, EncodingNotUTF8, "LANG=en_US.UTF-16 names the codeset UTF-16, not UTF-8"},
		{"POSIX", env{"LANG": "POSIX"}, EncodingNotUTF8, "LANG=POSIX is the C locale, whose characters are ASCII"},

		{"no codeset", env{"LANG": "de_DE"}, EncodingUnknown, "LANG=de_DE names no codeset, and the default for a locale differs from one system to the next"},
		{"no codeset, with a modifier", env{"LANG": "de_DE@euro"}, EncodingUnknown, "LANG=de_DE@euro names no codeset, and the default for a locale differs from one system to the next"},
		{"a dot and nothing after it", env{"LANG": "en_US."}, EncodingUnknown, "LANG=en_US. names no codeset, and the default for a locale differs from one system to the next"},
		{"nothing set", env{}, EncodingUnknown, "none of LC_ALL, LC_CTYPE and LANG is set"},
		{"only variables the check does not read", env{"TERM": "xterm-256color", "LC_MESSAGES": "en_US.UTF-8"}, EncodingUnknown, "none of LC_ALL, LC_CTYPE and LANG is set"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := judge(outTerminal, 0, tc.env.get)
			if got.Encoding != tc.want {
				t.Errorf("Encoding = %v, want %v", got.Encoding, tc.want)
			}
			if got.Reason.String() != tc.wantReason {
				t.Errorf("Reason = %q, want %q", got.Reason.String(), tc.wantReason)
			}
			checkShowable(t, got)
		})
	}
}

// TestJudge_Evidence: everything looked at is listed in the order it was
// looked at, and the search stops at the first locale variable that is set.
func TestJudge_Evidence(t *testing.T) {
	cases := []struct {
		name string
		kind output
		page uint32
		env  env
		want []string
	}{
		{"a terminal, LANG only", outTerminal, 0, env{"LANG": "en_US.UTF-8"}, []string{
			"the output is a character device, as a terminal is",
			"LC_ALL is not set",
			"LC_CTYPE is not set",
			"LANG=en_US.UTF-8",
		}},
		{"LC_ALL set: the others are not looked at", outTerminal, 0, env{"LC_ALL": "C", "LANG": "en_US.UTF-8"}, []string{
			"the output is a character device, as a terminal is",
			"LC_ALL=C",
		}},
		{"a pipe, nothing set", outNotTerminal, 0, env{}, []string{
			"the output is not a terminal but a pipe or a file, and what will draw it is not known",
			"LC_ALL is not set",
			"LC_CTYPE is not set",
			"LANG is not set",
		}},
		{"no output", outNone, 0, env{"LC_CTYPE": "UTF-8"}, []string{
			"the output could not be examined",
			"LC_ALL is not set",
			"LC_CTYPE=UTF-8",
		}},
		{"over ssh, inside tmux", outTerminal, 0, env{"LANG": "en_US.UTF-8", "SSH_CONNECTION": "10.0.0.1 50000 10.0.0.2 22", "TMUX": "/tmp/tmux-1000/default,1,0"}, []string{
			"the output is a character device, as a terminal is",
			"LC_ALL is not set",
			"LC_CTYPE is not set",
			"LANG=en_US.UTF-8",
			"this is an ssh session: the terminal is on another machine, which may not use this locale",
			"TMUX is set: a terminal multiplexer draws between this program and the terminal",
		}},
		{"ssh known by SSH_TTY, inside screen", outTerminal, 0, env{"LANG": "C", "SSH_TTY": "/dev/pts/3", "STY": "1234.pts-0.host"}, []string{
			"the output is a character device, as a terminal is",
			"LC_ALL is not set",
			"LC_CTYPE is not set",
			"LANG=C",
			"this is an ssh session: the terminal is on another machine, which may not use this locale",
			"STY is set: a terminal multiplexer draws between this program and the terminal",
		}},
		{"Windows, not a console, under MSYS", outNotConsole, 0, env{"LANG": "en_US.UTF-8"}, []string{
			"the output is not a Windows console but a pipe, a file, or a terminal that Windows presents as a pipe",
			"LC_ALL is not set",
			"LC_CTYPE is not set",
			"LANG=en_US.UTF-8",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := judge(tc.kind, tc.page, tc.env.get)
			var lines []string
			for _, e := range got.Evidence {
				lines = append(lines, e.String())
			}
			if strings.Join(lines, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("Evidence:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(tc.want, "\n"))
			}
			checkShowable(t, got)
		})
	}
}

// TestJudge_WindowsConsole: Go writes to a console in UTF-16, so the code
// page is evidence and does not decide, and the locale variables are not
// consulted.
func TestJudge_WindowsConsole(t *testing.T) {
	for _, page := range []uint32{437, 850, 932, 1252, 65001} {
		got := judge(outConsole, page, env{"LANG": "C"}.get)
		if got.Encoding != EncodingUTF8 {
			t.Errorf("code page %d: Encoding = %v, want UTF-8", page, got.Encoding)
		}
		if want := "the output is a Windows console, which Go writes to in UTF-16 whatever its code page"; got.Reason.String() != want {
			t.Errorf("code page %d: Reason = %q, want %q", page, got.Reason.String(), want)
		}
		if len(got.Evidence) != 1 || !strings.HasSuffix(got.Evidence[0].String(), "output code page "+itoa(page)) {
			t.Errorf("code page %d: Evidence = %v", page, got.Evidence)
		}
		checkShowable(t, got)
	}
}

func itoa(n uint32) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for ; n > 0; n /= 10 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
	}
	return string(digits)
}

// TestJudge_HostileEnvironment: the environment is outside text. A locale
// value is quoted in the reason and the evidence, so it is cleaned, made
// ASCII and cut short first.
func TestJudge_HostileEnvironment(t *testing.T) {
	for _, s := range Samples() {
		for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
			for _, value := range []string{s.Text, "en_US." + s.Text, s.Text + ".UTF-8", "en_US.UTF-8@" + s.Text} {
				got := judge(outTerminal, 0, env{name: value, "TMUX": s.Text, "SSH_TTY": s.Text}.get)
				checkShowable(t, got)
			}
		}
	}

	long := judge(outTerminal, 0, env{"LANG": strings.Repeat("x", 500) + ".ISO-8859-1"}.get)
	if n := len(long.Reason.String()); n > 200 {
		t.Errorf("a long value makes a reason of %d bytes: %q", n, long.Reason.String())
	}
	if long.Encoding != EncodingNotUTF8 {
		t.Errorf("a long value: Encoding = %v", long.Encoding)
	}
}

// checkShowable: a program prints the reason and the evidence, on a terminal
// it may just have learned is not UTF-8. They are one line of printable ASCII
// each, with nothing left in them to clean.
func checkShowable(t *testing.T, c Terminal) {
	t.Helper()
	if c.Reason.IsEmpty() {
		t.Error("no reason given")
	}
	if len(c.Evidence) == 0 {
		t.Error("no evidence given")
	}
	for _, line := range append([]Text{c.Reason}, c.Evidence...) {
		s := line.String()
		if b, bad := firstNotASCII(s, true); bad {
			t.Errorf("%q holds the byte %#x", s, b)
		}
		if again, rep := Mark.LineReport(s); rep.Changed() || again != line {
			t.Errorf("%q still has something to clean: %+v", s, rep)
		}
	}
}

func TestTerminal_Fit(t *testing.T) {
	text := Drop.Line("caf" + u(0x00E9))
	for _, tc := range []struct {
		terminal Terminal
		want     string
	}{
		{Terminal{Encoding: EncodingUTF8}, "caf" + u(0x00E9)},
		{Terminal{Encoding: EncodingNotUTF8}, "caf<U+00E9>"},
		{Terminal{Encoding: EncodingUnknown}, "caf<U+00E9>"},
		{Terminal{}, "caf<U+00E9>"}, // a Terminal nobody filled in draws ASCII
	} {
		if got := tc.terminal.Fit(text).String(); got != tc.want {
			t.Errorf("%v: Fit = %+q, want %+q", tc.terminal.Encoding, got, tc.want)
		}
	}
}

func TestEncoding_String(t *testing.T) {
	for e, want := range map[Encoding]string{
		EncodingUTF8:    "UTF-8",
		EncodingNotUTF8: "not UTF-8",
		EncodingUnknown: "cannot tell",
		Encoding(42):    "cannot tell",
	} {
		if got := e.String(); got != want {
			t.Errorf("Encoding(%d).String() = %q, want %q", int(e), got, want)
		}
	}
	var zero Encoding
	if zero != EncodingUnknown {
		t.Error("the zero Encoding is not EncodingUnknown")
	}
}
