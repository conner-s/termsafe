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
//   - On Windows, what out is. For a console (Windows Terminal, the classic
//     console window, a ConPTY under an editor or under sshd) the verdict is
//     EncodingUTF8, because package os writes to a console in UTF-16, with
//     WriteConsoleW, and no code page decodes the text. The code page and
//     the console mode go into the evidence. This holds for writes through
//     an *os.File; bytes that reach the console another way, a child
//     process's output for one, are decoded by the code page, and only 65001
//     is UTF-8. For an MSYS or Cygwin terminal (mintty, Git Bash), which
//     Windows presents as a pipe, bytes go out as written and the locale
//     variables decide, as above. So they do for any other pipe and for a
//     file, where they are usually not set and the verdict is
//     EncodingUnknown.
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
// The check does not ask the terminal. [Terminal.Probe] does, for a program
// that owns the terminal and calls it: its answer outranks this one.
func CheckTerminal(out *os.File) Terminal {
	return judge(examineOutput(out), os.Getenv)
}

// output is what kind of thing a program draws on.
type output int

const (
	outNone        output = iota // nil, or it could not be examined
	outTerminal                  // not Windows: a character device
	outNotTerminal               // not Windows: a pipe or a file
	outConsole                   // Windows: a console, as GetConsoleMode tells
	outPipe                      // Windows: a pipe
	outFile                      // Windows: a file, or a device that is no console
)

// outputFacts is what the operating system says about the file a program
// draws on. examineOutput gathers it with the system calls of one platform;
// judge decides from it and is the same code on every platform, so the whole
// decision is tested everywhere, the Windows rows too.
type outputFacts struct {
	kind output
	// consoleMode and codePage are GetConsoleMode and GetConsoleOutputCP,
	// for a Windows console.
	consoleMode uint32
	codePage    uint32
	// pipeName is the name of a Windows pipe, as
	// GetFileInformationByHandleEx gives it.
	pipeName string
}

// vtProcessing is ENABLE_VIRTUAL_TERMINAL_PROCESSING: the console obeys
// escape sequences in what is written to it.
const vtProcessing = 0x0004

// judge is CheckTerminal without the operating system.
//
// The rows for Windows follow from how package os writes, which is the same
// in Go 1.26 and Go 1.27. os.newFile (os/file_windows.go) calls a handle a
// console when syscall.GetConsoleMode succeeds on it. For a console,
// (*FD).Write (internal/poll/fd_windows.go) calls (*FD).writeConsole, which
// decodes the bytes as UTF-8 and hands the characters to syscall.WriteConsole,
// that is WriteConsoleW, in UTF-16: no byte reaches the console, so no code
// page decodes one. For a pipe or a file the same Write calls
// syscall.WriteFile with the bytes as they are, and whatever reads them
// decides what they mean.
func judge(facts outputFacts, getenv func(string) string) Terminal {
	var c Terminal
	note := func(line string) {
		c.Evidence = append(c.Evidence, ownWords(line))
	}

	byLocale := true
	switch facts.kind {
	case outConsole:
		// Windows Terminal, the classic console window, a ConPTY under
		// an editor's terminal or under sshd: all hand the program a
		// console handle.
		vt := "off, so it draws escape sequences instead of obeying them"
		if facts.consoleMode&vtProcessing != 0 {
			vt = "on"
		}
		note(fmt.Sprintf("the output is a Windows console: output code page %d, virtual terminal processing %s", facts.codePage, vt))
		c.Encoding = EncodingUTF8
		c.Reason = ownWords("the output is a Windows console, which Go writes to in UTF-16 (WriteConsoleW), so no code page decodes it")
		byLocale = false
	case outPipe:
		if isPTYName(facts.pipeName) {
			// mintty, Git Bash and the Cygwin terminal: a pty that
			// Windows shows as a pipe. Bytes go to the terminal as
			// written, and MSYS and Cygwin export the locale it uses.
			note("the output is an MSYS or Cygwin terminal, which Windows presents as the pipe " + shown(facts.pipeName))
		} else {
			note("the output is a pipe, and what reads it is not known")
		}
	case outFile:
		note("the output is a file or a device that is not a console, and what will draw it is not known")
	case outTerminal:
		note("the output is a character device, as a terminal is")
	case outNotTerminal:
		note("the output is not a terminal but a pipe or a file, and what will draw it is not known")
	default:
		note("the output could not be examined")
	}

	if byLocale {
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

// isPTYName reports whether a Windows pipe name is that of an MSYS or Cygwin
// pty: \msys-1888ae32e00d56aa-pty0-to-master, or the same with cygwin, and
// with from-master for input.
func isPTYName(name string) bool {
	name = strings.TrimPrefix(name, `\`)
	if !strings.HasPrefix(name, "msys-") && !strings.HasPrefix(name, "cygwin-") {
		return false
	}
	_, rest, found := strings.Cut(name, "-pty")
	return found && (strings.HasSuffix(rest, "-to-master") || strings.HasSuffix(rest, "-from-master"))
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
