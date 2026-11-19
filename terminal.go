package termsafe

import (
	"fmt"
	"os"
	"strings"
)

// An Encoding is what can be said about the character encoding of the
// terminal a program draws on.
type Encoding int

const (
	// EncodingUnknown means nothing found says which encoding the terminal
	// uses. It is the zero value, and it is treated like EncodingNotUTF8:
	// only ASCII is drawn.
	EncodingUnknown Encoding = iota
	// EncodingUTF8 means the terminal can be taken as UTF-8 as far as the
	// environment says. It is not proof: see [CheckTerminal].
	EncodingUTF8
	// EncodingNotUTF8 means the environment names another encoding.
	EncodingNotUTF8
)

// String returns "UTF-8", "not UTF-8" or "cannot tell".
func (e Encoding) String() string {
	switch e {
	case EncodingUTF8:
		return "UTF-8"
	case EncodingNotUTF8:
		return "not UTF-8"
	}
	return "cannot tell"
}

// A Terminal is what [CheckTerminal] found out about the terminal a program
// draws on. The zero value knows nothing, and [Terminal.Fit] then draws
// ASCII only.
type Terminal struct {
	// Encoding is the verdict.
	Encoding Encoding
	// Reason is the one finding that decided it, in words a program can
	// show its user, such as "LANG=en_US.UTF-8 names UTF-8".
	Reason Text
	// Evidence is everything that was looked at, in the order it was
	// looked at, the things that weaken the verdict included: an ssh
	// session, a terminal multiplexer, an output that is not a terminal.
	Evidence []Text
}

// Fit returns t as this terminal can take it: t itself when the terminal can
// be taken as UTF-8, and [Text.ASCII] of it otherwise, also when nothing is
// known. Use it at each place a program turns outside text into a Text.
func (c Terminal) Fit(t Text) Text {
	if c.Encoding == EncodingUTF8 {
		return t
	}
	return t.ASCII()
}

// CheckTerminal says whether the terminal behind out, the file a program is
// about to draw on (os.Stdout for most), can be taken to decode UTF-8. Call
// it once at start, draw every Text through [Terminal.Fit], and tell the user
// when the verdict is not UTF-8: Reason says why.
//
// It matters because everything a [Text] promises assumes UTF-8. A terminal
// in an 8-bit mode reads the bytes 0x80 to 0x9F as C1 controls, and ordinary
// characters hold those bytes in their UTF-8 encoding.
//
// The check reads the environment and writes nothing. It looks at:
//
//   - The locale, as a C library would: LC_ALL if it is set and not empty,
//     else LC_CTYPE, else LANG. A codeset of UTF-8 (en_US.UTF-8, C.utf8, or
//     macOS's bare LC_CTYPE=UTF-8) gives EncodingUTF8. Any other codeset, and
//     the C and POSIX locales, give EncodingNotUTF8. A locale that names no
//     codeset (de_DE) gives EncodingUnknown, because its default differs
//     from one system to the next, and so does no locale at all.
//   - On Windows, whether out is a console. Go writes to a console through
//     WriteConsoleW, in UTF-16, so UTF-8 text arrives whole whatever the
//     console's output code page is: EncodingUTF8, with the code page in the
//     evidence. This holds for writes through an *os.File. Bytes written to
//     the handle another way, and the output of child processes, are decoded
//     by the code page, and only 65001 is UTF-8. When out is not a console
//     (a pipe, a file, or mintty and other terminals that Windows presents
//     as a pipe) the locale variables decide, as above.
//   - Whether out is a terminal at all, SSH_CONNECTION and SSH_TTY, and TMUX
//     and STY. These go into the evidence and do not change the verdict.
//
// What it cannot know: whether the terminal emulator is set to the encoding
// the locale names; whether the terminal at the far end of an ssh session
// honours the locale of this end; what a multiplexer or a serial line in
// between does; and what will draw output that goes into a pipe or a file.
// EncodingUTF8 therefore means "UTF-8 as far as the environment says". A
// program should give its user a way to overrule the verdict in both
// directions.
//
// The check does not ask the terminal. That would mean writing a character
// and a cursor position query to it and reading the reply from its input,
// which belongs to the program or its TUI framework. A program that owns the
// terminal can do so: README.md says how.
func CheckTerminal(out *os.File) Terminal {
	kind, codePage := outputKind(out)
	return judge(kind, codePage, os.Getenv)
}

// output is what kind of thing a program draws on.
type output int

const (
	outNone        output = iota // nil, or it could not be examined
	outTerminal                  // a character device
	outNotTerminal               // a pipe or a file
	outConsole                   // a Windows console
	outNotConsole                // on Windows, anything else
)

// judge is CheckTerminal without the operating system: kind and codePage are
// what outputKind found, getenv reads the environment.
func judge(kind output, codePage uint32, getenv func(string) string) Terminal {
	var c Terminal
	note := func(line string) {
		c.Evidence = append(c.Evidence, ownWords(line))
	}

	switch kind {
	case outConsole:
		note(fmt.Sprintf("the output is a Windows console with output code page %d", codePage))
		c.Encoding = EncodingUTF8
		c.Reason = ownWords("the output is a Windows console, which Go writes to in UTF-16 whatever its code page")
		return c
	case outTerminal:
		note("the output is a character device, as a terminal is")
	case outNotTerminal:
		note("the output is not a terminal but a pipe or a file, and what will draw it is not known")
	case outNotConsole:
		note("the output is not a Windows console but a pipe, a file, or a terminal that Windows presents as a pipe")
	default:
		note("the output could not be examined")
	}

	c.Encoding = EncodingUnknown
	c.Reason = ownWords("none of LC_ALL, LC_CTYPE and LANG is set")
	for _, name := range [...]string{"LC_ALL", "LC_CTYPE", "LANG"} {
		value := getenv(name)
		if value == "" {
			note(name + " is not set")
			continue
		}
		setting := name + "=" + shown(value)
		note(setting)
		encoding, why := codeset(value)
		c.Encoding = encoding
		c.Reason = ownWords(setting + " " + why)
		break
	}

	if getenv("SSH_CONNECTION") != "" || getenv("SSH_TTY") != "" {
		note("this is an ssh session: the terminal is on another machine, which may not use this locale")
	}
	for _, name := range [...]string{"TMUX", "STY"} {
		if getenv(name) != "" {
			note(name + " is set: a terminal multiplexer draws between this program and the terminal")
		}
	}
	return c
}

// codeset reads a locale name, language[_territory][.codeset][@modifier], and
// says what its codeset means, in words that follow "NAME=value".
func codeset(locale string) (Encoding, string) {
	name, _, _ := strings.Cut(locale, "@")
	_, set, named := strings.Cut(name, ".")
	switch {
	case named && isUTF8(set), !named && isUTF8(name):
		return EncodingUTF8, "names UTF-8"
	case named && set != "":
		return EncodingNotUTF8, "names the codeset " + shown(set) + ", not UTF-8"
	case name == "C" || name == "POSIX":
		return EncodingNotUTF8, "is the C locale, whose characters are ASCII"
	}
	return EncodingUnknown, "names no codeset, and the default for a locale differs from one system to the next"
}

// isUTF8 reports whether a codeset name means UTF-8, however it is spelled:
// UTF-8, utf8, UTF_8.
func isUTF8(set string) bool {
	set = strings.NewReplacer("-", "", "_", "").Replace(set)
	return strings.EqualFold(set, "utf8")
}

// shown makes a value from the environment safe to quote in evidence. The
// environment is outside text like any other.
func shown(value string) string {
	return Drop.Line(value).ASCII().Truncate(40).String()
}

// ownWords makes a Text of a sentence this file wrote.
func ownWords(line string) Text {
	return Drop.Line(line).ASCII()
}
