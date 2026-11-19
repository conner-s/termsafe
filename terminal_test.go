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
			got := judge(outputFacts{kind: outTerminal}, tc.env.get)
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
		env  env
		want []string
	}{
		{"a terminal, LANG only", outTerminal, env{"LANG": "en_US.UTF-8"}, []string{
			"the output is a character device, as a terminal is",
			"LC_ALL is not set",
			"LC_CTYPE is not set",
			"LANG=en_US.UTF-8",
		}},
		{"LC_ALL set: the others are not looked at", outTerminal, env{"LC_ALL": "C", "LANG": "en_US.UTF-8"}, []string{
			"the output is a character device, as a terminal is",
			"LC_ALL=C",
		}},
		{"a pipe, nothing set", outNotTerminal, env{}, []string{
			"the output is not a terminal but a pipe or a file, and what will draw it is not known",
			"LC_ALL is not set",
			"LC_CTYPE is not set",
			"LANG is not set",
		}},
		{"no output", outNone, env{"LC_CTYPE": "UTF-8"}, []string{
			"the output could not be examined",
			"LC_ALL is not set",
			"LC_CTYPE=UTF-8",
		}},
		{"over ssh, inside tmux", outTerminal, env{"LANG": "en_US.UTF-8", "SSH_CONNECTION": "10.0.0.1 50000 10.0.0.2 22", "TMUX": "/tmp/tmux-1000/default,1,0"}, []string{
			"the output is a character device, as a terminal is",
			"LC_ALL is not set",
			"LC_CTYPE is not set",
			"LANG=en_US.UTF-8",
			"this is an ssh session: the terminal is on another machine, which may not use this locale",
			"TMUX is set: a terminal multiplexer draws between this program and the terminal",
		}},
		{"ssh known by SSH_TTY, inside screen", outTerminal, env{"LANG": "C", "SSH_TTY": "/dev/pts/3", "STY": "1234.pts-0.host"}, []string{
			"the output is a character device, as a terminal is",
			"LC_ALL is not set",
			"LC_CTYPE is not set",
			"LANG=C",
			"this is an ssh session: the terminal is on another machine, which may not use this locale",
			"STY is set: a terminal multiplexer draws between this program and the terminal",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := judge(outputFacts{kind: tc.kind}, tc.env.get)
			if lines := evidenceLines(got); lines != strings.Join(tc.want, "\n") {
				t.Errorf("Evidence:\n%s\nwant:\n%s", lines, strings.Join(tc.want, "\n"))
			}
			checkShowable(t, got)
		})
	}
}

func evidenceLines(c Terminal) string {
	var lines []string
	for _, e := range c.Evidence {
		lines = append(lines, e.String())
	}
	return strings.Join(lines, "\n")
}

// The names Windows gives the pipes of an MSYS and a Cygwin pty.
const (
	msysPTY   = `\msys-1888ae32e00d56aa-pty0-to-master`
	cygwinPTY = `\cygwin-e022582115c10879-pty12-to-master`
)

// TestJudge_Windows is the decision table for Windows. It runs on every
// platform: the facts are given here, where on Windows examineOutput reads
// them from the system. Each row says what the output is, what the
// environment holds, and the verdict that follows from how package os writes
// to that kind of handle.
func TestJudge_Windows(t *testing.T) {
	const (
		console    = "the output is a Windows console, which Go writes to in UTF-16 (WriteConsoleW), so no code page decodes it"
		notSet     = "none of LC_ALL, LC_CTYPE and LANG is set"
		isPTY      = "the output is an MSYS or Cygwin terminal, which Windows presents as the pipe "
		plainPipe  = "the output is a pipe, and what reads it is not known"
		fileOrNUL  = "the output is a file or a device that is not a console, and what will draw it is not known"
		overSSH    = "this is an ssh session: the terminal is on another machine, which may not use this locale"
		vtOn       = "virtual terminal processing on"
		vtOff      = "virtual terminal processing off, so it draws escape sequences instead of obeying them"
		someLocale = "LANG=en_US.UTF-8"
	)
	cases := []struct {
		name         string
		facts        outputFacts
		env          env
		want         Encoding
		wantReason   string
		wantEvidence []string // each must be one of the evidence lines, or the start of one
		notEvidence  string   // must be in none of them
	}{
		// A console handle. Go writes UTF-16 through WriteConsoleW; the
		// code page plays no part and the locale variables are not read.
		{
			name:         "Windows Terminal or a ConPTY: a console with virtual terminal processing",
			facts:        outputFacts{kind: outConsole, consoleMode: 0x0007, codePage: 437},
			env:          env{},
			want:         EncodingUTF8,
			wantReason:   console,
			wantEvidence: []string{"the output is a Windows console: output code page 437, " + vtOn},
			notEvidence:  "LANG",
		},
		{
			name:         "the classic console, virtual terminal processing off",
			facts:        outputFacts{kind: outConsole, consoleMode: 0x0003, codePage: 850},
			env:          env{},
			want:         EncodingUTF8,
			wantReason:   console,
			wantEvidence: []string{"the output is a Windows console: output code page 850, " + vtOff},
		},
		{
			name:         "a console set to UTF-8: the same verdict, for the same reason",
			facts:        outputFacts{kind: outConsole, consoleMode: 0x0007, codePage: 65001},
			env:          env{},
			want:         EncodingUTF8,
			wantReason:   console,
			wantEvidence: []string{"the output is a Windows console: output code page 65001, " + vtOn},
		},
		{
			name:         "a Japanese console",
			facts:        outputFacts{kind: outConsole, consoleMode: 0x0007, codePage: 932},
			env:          env{},
			want:         EncodingUTF8,
			wantReason:   console,
			wantEvidence: []string{"output code page 932"},
		},
		{
			name:        "a console with a locale in the environment that says otherwise: the console decides",
			facts:       outputFacts{kind: outConsole, consoleMode: 0x0007, codePage: 437},
			env:         env{"LANG": "C", "LC_ALL": "en_US.ISO-8859-1"},
			want:        EncodingUTF8,
			wantReason:  console,
			notEvidence: "LC_ALL",
		},
		{
			name:         "ssh into Windows: sshd gives the shell a ConPTY, so a console, and the far terminal is not known",
			facts:        outputFacts{kind: outConsole, consoleMode: 0x0007, codePage: 437},
			env:          env{"SSH_CONNECTION": "10.0.0.1 50000 10.0.0.2 22"},
			want:         EncodingUTF8,
			wantReason:   console,
			wantEvidence: []string{"the output is a Windows console", overSSH},
		},

		// Not a console. Go writes the bytes as they are with WriteFile.
		// A pty of MSYS or Cygwin is a terminal that decodes them by the
		// locale its shell exported.
		{
			name:         "mintty or Git Bash: a pty, with the locale MSYS exports",
			facts:        outputFacts{kind: outPipe, pipeName: msysPTY},
			env:          env{"LANG": "en_US.UTF-8"},
			want:         EncodingUTF8,
			wantReason:   "LANG=en_US.UTF-8 names UTF-8",
			wantEvidence: []string{isPTY + msysPTY, "LC_ALL is not set", someLocale},
		},
		{
			name:         "a Cygwin terminal set to another charset",
			facts:        outputFacts{kind: outPipe, pipeName: cygwinPTY},
			env:          env{"LC_ALL": "ja_JP.SJIS"},
			want:         EncodingNotUTF8,
			wantReason:   "LC_ALL=ja_JP.SJIS names the codeset SJIS, not UTF-8",
			wantEvidence: []string{isPTY + cygwinPTY},
		},
		{
			name:         "a pty with no locale exported",
			facts:        outputFacts{kind: outPipe, pipeName: msysPTY},
			env:          env{},
			want:         EncodingUnknown,
			wantReason:   notSet,
			wantEvidence: []string{isPTY, "LANG is not set"},
		},
		{
			name:         "a pty reached over ssh",
			facts:        outputFacts{kind: outPipe, pipeName: cygwinPTY},
			env:          env{"LANG": "C.UTF-8", "SSH_TTY": "/dev/pty1"},
			want:         EncodingUTF8,
			wantReason:   "LANG=C.UTF-8 names UTF-8",
			wantEvidence: []string{isPTY, overSSH},
		},
		{
			name:         "piped into another program",
			facts:        outputFacts{kind: outPipe, pipeName: ""},
			env:          env{},
			want:         EncodingUnknown,
			wantReason:   notSet,
			wantEvidence: []string{plainPipe},
			notEvidence:  "MSYS",
		},
		{
			name:         "piped into another program from an MSYS shell",
			facts:        outputFacts{kind: outPipe, pipeName: `\pipe-not-a-pty`},
			env:          env{"LANG": "en_US.UTF-8"},
			want:         EncodingUTF8,
			wantReason:   "LANG=en_US.UTF-8 names UTF-8",
			wantEvidence: []string{plainPipe, someLocale},
		},
		{
			name:         "redirected to a file, or to NUL",
			facts:        outputFacts{kind: outFile},
			env:          env{},
			want:         EncodingUnknown,
			wantReason:   notSet,
			wantEvidence: []string{fileOrNUL},
		},
		{
			name:         "a handle Windows would not describe",
			facts:        outputFacts{kind: outNone},
			env:          env{},
			want:         EncodingUnknown,
			wantReason:   notSet,
			wantEvidence: []string{"the output could not be examined"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := judge(tc.facts, tc.env.get)
			if got.Encoding != tc.want {
				t.Errorf("Encoding = %v, want %v", got.Encoding, tc.want)
			}
			if got.Reason.String() != tc.wantReason {
				t.Errorf("Reason = %q, want %q", got.Reason.String(), tc.wantReason)
			}
			lines := evidenceLines(got)
			for _, want := range tc.wantEvidence {
				if !strings.Contains(lines, want) {
					t.Errorf("Evidence lacks %q:\n%s", want, lines)
				}
			}
			if tc.notEvidence != "" && strings.Contains(lines, tc.notEvidence) {
				t.Errorf("Evidence mentions %q:\n%s", tc.notEvidence, lines)
			}
			checkShowable(t, got)
		})
	}
}

func TestIsPTYName(t *testing.T) {
	for name, want := range map[string]bool{
		msysPTY:   true,
		cygwinPTY: true,
		`\msys-1888ae32e00d56aa-pty3-from-master`: true,
		`msys-1888ae32e00d56aa-pty0-to-master`:    true, // without the backslash
		"":                                        false,
		`\pipe-not-a-pty`:                         false,
		`\msys-1888ae32e00d56aa-pipe-1`:           false,
		`\msys-1888ae32e00d56aa-pty0`:             false,
		`\other-1888ae32-pty0-to-master`:          false,
		`\LOCAL\crashpad_1234_ABCDEFGH`:           false,
		`\mojo.1234.5678.9`:                       false,
	} {
		if got := isPTYName(name); got != want {
			t.Errorf("isPTYName(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestJudge_HostilePipeName: the name of a pipe is outside text, like the
// environment. It is quoted in the evidence only cleaned and cut short.
func TestJudge_HostilePipeName(t *testing.T) {
	for _, s := range Samples() {
		got := judge(outputFacts{kind: outPipe, pipeName: `\msys-` + s.Text + "-pty0-to-master"}, env{}.get)
		checkShowable(t, got)
	}
}

// TestJudge_HostileEnvironment: the environment is outside text. A locale
// value is quoted in the reason and the evidence, so it is cleaned, made
// ASCII and cut short first.
func TestJudge_HostileEnvironment(t *testing.T) {
	for _, s := range Samples() {
		for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
			for _, value := range []string{s.Text, "en_US." + s.Text, s.Text + ".UTF-8", "en_US.UTF-8@" + s.Text} {
				got := judge(outputFacts{kind: outTerminal, pipeName: value}, env{name: value, "TMUX": s.Text, "SSH_TTY": s.Text}.get)
				checkShowable(t, got)
			}
		}
	}

	long := judge(outputFacts{kind: outTerminal}, env{"LANG": strings.Repeat("x", 500) + ".ISO-8859-1"}.get)
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
