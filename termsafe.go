// Package termsafe turns text a program did not write (a subprocess's output,
// a remote title, a file name, an error quoting any of those) into text that
// is safe to draw in a terminal.
//
// A terminal acts on control sequences wherever they turn up, and a TUI
// framework may pass them through: bubbletea 2.1 replays terminated DCS, APC,
// PM and SOS strings found in a view's text. Text that reaches the screen
// must therefore not hold any. Text can also mislead without controlling
// anything: a bidirectional override reorders what the reader sees, an
// invisible character makes two different names look the same. The package
// deals with both.
//
// It has one type, [Text], and one way to make it: clean a string under a
// [Policy]. The policy is the caller's answer to one question, what should
// happen to bidirectional controls and invisible characters:
//
//	termsafe.Drop.Line(s)  // remove them: a log, a progress line
//	termsafe.Mark.Line(s)  // show each as <U+202E>: text a person judges
//
// and each policy cleans in two shapes:
//
//	p.Clean(s) // may span lines: newlines and tabs are kept
//	p.Line(s)  // one line: newlines and tabs become spaces
//
// There is no default policy and no way to clean without naming one. A Text
// cannot be built any other way outside this package, so a function that
// takes a Text cannot be handed a raw string by accident. [Policy.CleanReport]
// and [Policy.LineReport] do the same and also say what they took out, for a
// program that tells its user when it changed something. Use the package like
// this:
//
//   - Make every setter and message field that carries outside text a Text.
//   - Clean once, where the text enters the UI: where a subprocess line, an
//     error or a path is wrapped into a message for the views, or just before
//     it is printed.
//   - Keep the raw string for everything that is not display (opening the
//     file, the clipboard, parsing). A Text is for the screen only.
//
// # What a Text never holds
//
//   - Bytes that are not valid UTF-8.
//   - A C0 control (U+0000 to U+001F), DEL or a C1 control (U+0080 to
//     U+009F), except that Clean keeps newline and tab. With no ESC and no C1
//     in it, no control sequence can be in it, and none can form when two
//     results are joined.
//   - A bidirectional control or an invisible format character (listed
//     below), under either policy.
//
// Printable text, non-ASCII included, is unchanged.
//
// # Control sequences
//
// What is removed, and how much of it:
//
//   - CSI sequences (ESC [ or U+009B, then parameters and a final byte),
//     whole.
//   - OSC, DCS, APC, PM and SOS strings (ESC ] P _ ^ X or their C1 forms),
//     with their payload, up to the terminator: ST (ESC \ or U+009C), or BEL
//     for OSC. The text of an OSC 8 hyperlink is ordinary text between two
//     such strings and stays.
//   - An unterminated string, up to the end of its line. A sequence never
//     reaches past a newline, so cleaning a + "\n" + b gives the cleaned a, a
//     newline, the cleaned b: it does not matter how a reader split its input
//     into lines, and hostile text can hide no more than the rest of its own
//     line.
//   - Other escape sequences (ESC c, ESC 7, ESC ( B and the like), whole.
//   - Every other control character, by itself.
//
// CR LF becomes a newline, and so does a lone CR: it is how progress output
// separates the states of a line it redraws.
//
// A byte that is not valid UTF-8 becomes U+FFFD and what follows it stays as
// text. That covers the 8-bit forms of the C1 controls (0x80 to 0x9F as single
// bytes), which a terminal outside UTF-8 mode acts on. They are not read as
// sequence introducers because the same bytes are letters in the legacy code
// pages a tool may write on Windows, and reading one as DCS would swallow the
// rest of the line.
//
// # Bidirectional controls and invisible characters
//
// These are dropped or marked, as the policy says, and counted in the Report
// either way:
//
//   - The bidirectional controls: the embeddings and overrides U+202A to
//     U+202E, the isolates U+2066 to U+2069, and the marks U+200E, U+200F and
//     U+061C. An override or an isolate makes a terminal that implements the
//     bidirectional algorithm draw the characters after it in another order
//     than they are stored in ("Trojan Source"); the marks are invisible and
//     can do the same to neighbouring punctuation.
//   - Invisible format characters: the zero width space U+200B, the word
//     joiner U+2060, the byte order mark U+FEFF, the soft hyphen U+00AD, the
//     invisible operators U+2061 to U+2064, the deprecated format characters
//     U+206A to U+206F, the Mongolian vowel separator U+180E, the interlinear
//     annotation characters U+FFF9 to U+FFFB, the Hangul fillers, the
//     combining grapheme joiner U+034F, and the rest of Unicode's format (Cf)
//     and default-ignorable characters. None draws anything, and each makes
//     two strings differ where a reader sees no difference.
//   - The joiners U+200C and U+200D where they can have no effect: at the
//     start of the text, or after an ASCII character, a Latin, Greek or
//     Cyrillic letter, or white space ("pay", U+200D, "pal").
//   - Tag characters U+E0020 to U+E007F outside an emoji flag. They are
//     invisible, and a run of them can carry a whole hidden sentence.
//
// These are left alone, because legitimate text needs them: a joiner after
// any other character (the zero width joiner builds emoji such as a family
// or a profession, and Arabic, Persian and Indic scripts use both joiners);
// variation selectors (emoji and ideograph presentation); tag characters
// that follow U+1F3F4, which spell the flags of England, Scotland and Wales;
// the Arabic number signs, which are format characters that do draw. Text in
// a right-to-left script is ordinary text and is never touched.
//
// # Look-alike characters
//
// [Text.MixedScripts] finds words whose letters come from scripts that do not
// belong together, such as a Latin word with one Greek or Cyrillic letter in
// it. It only reports: the text is never altered, because a name must stay
// what it is. The standard library has the script of every character but not
// Unicode's table of confusable characters, so this is the check that is
// possible without one. It does not find a word written wholly in another
// script (an all-Cyrillic word that reads as a Latin one), look-alikes inside
// one script (l and I, rn and m), or compatibility forms that Unicode files
// under Latin or under no script (fullwidth letters, mathematical letters).
// Where a name must be ASCII, as package names in most registries must,
// testing for that is stricter and simpler than any of this.
//
// The package depends on the standard library only and nothing in it is
// specific to one program: copy the directory.
package termsafe

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Text is text that is safe to draw in a terminal. The zero value is the
// empty text. Outside this package a Text comes only from cleaning a string
// under a [Policy], or from the methods of another Text.
type Text struct {
	s string
}

// A Policy says what cleaning does with bidirectional controls and invisible
// format characters: the characters that control nothing in the terminal but
// can make text read differently from what it is. The package comment lists
// them. Everything else about cleaning is the same under both policies.
type Policy int

const (
	// Mark replaces each one with a visible marker that names it, such
	// as <U+202E>. The text then reads in the order it is stored, and the
	// reader sees that something was there and where. Use it for text a
	// person bases a decision on: a title, a name, a version, a diff.
	//
	// Mark is the zero value, so a Policy that was never set hides
	// nothing.
	Mark Policy = iota
	// Drop removes each one. The text reads normally and nothing shows
	// that it was touched; the Report still counts them. Use it where the
	// text is only watched go by, such as a tool's log, or where the
	// result is used and must not hold a marker, such as a URL.
	Drop
)

// Clean returns s made safe to draw, keeping its line structure: newlines and
// tabs stay. Use it for text that may span lines, such as an error.
func (p Policy) Clean(s string) Text {
	out, _ := clean(s, false, p)
	return Text{out}
}

// Line returns s made safe to draw as one line: newlines and tabs become
// spaces. Use it for a log line, a status, a file path or a URL.
func (p Policy) Line(s string) Text {
	out, _ := clean(s, true, p)
	return Text{out}
}

// CleanReport is [Policy.Clean], and also reports what was taken out of s.
func (p Policy) CleanReport(s string) (Text, Report) {
	out, rep := clean(s, false, p)
	return Text{out}, rep
}

// LineReport is [Policy.Line], and also reports what was taken out of s.
func (p Policy) LineReport(s string) (Text, Report) {
	out, rep := clean(s, true, p)
	return Text{out}, rep
}

// A Report counts what cleaning took out of one string. The zero Report means
// the text came through as it was, apart from white space: a carriage return
// made a newline, or Line's newlines and tabs made spaces, are not counted.
//
// A Report holds numbers only, never any of the bytes that were removed.
type Report struct {
	// Sequences is the number of control sequences removed whole: CSI
	// sequences, terminated OSC, DCS, APC, PM and SOS strings, and other
	// escape sequences.
	Sequences int
	// Unterminated is the number of OSC, DCS, APC, PM and SOS strings that
	// had no terminator. Each was cut where it stopped, at the end of its
	// line or at the next escape, and the text in between went with it:
	// this is the one case where text that might have been meant to be
	// read is lost.
	Unterminated int
	// Controls is the number of single control characters dropped: C0 and
	// C1 controls, DEL, and an ESC that began nothing.
	Controls int
	// Invalid is the number of bytes that were not valid UTF-8, each
	// replaced by U+FFFD.
	Invalid int
	// Bidi is the number of bidirectional controls: embeddings,
	// overrides, isolates and marks. They are counted whether the policy
	// dropped or marked them.
	Bidi int
	// Invisible is the number of invisible format characters, counted
	// whether the policy dropped or marked them.
	Invisible int
}

// Changed reports whether anything was removed, replaced or marked.
func (r Report) Changed() bool {
	return r != Report{}
}

// String describes the report in words, such as "2 control sequences and 1
// bidirectional control". It is empty when nothing changed. The result is
// plain ASCII and safe to draw.
func (r Report) String() string {
	var parts []string
	add := func(n int, one, many string) {
		switch {
		case n == 1:
			parts = append(parts, "1 "+one)
		case n > 1:
			parts = append(parts, fmt.Sprintf("%d %s", n, many))
		}
	}
	add(r.Sequences, "control sequence", "control sequences")
	add(r.Unterminated, "unterminated control string", "unterminated control strings")
	add(r.Controls, "control character", "control characters")
	add(r.Invalid, "invalid byte", "invalid bytes")
	add(r.Bidi, "bidirectional control", "bidirectional controls")
	add(r.Invisible, "invisible character", "invisible characters")
	return joinAnd(parts)
}

// String returns the text. It is safe to draw; it is not the original, so do
// not use it to open a file or as a URL.
func (t Text) String() string {
	return t.s
}

// IsEmpty reports whether there is no text.
func (t Text) IsEmpty() bool {
	return t.s == ""
}

// TrimSpace returns t without leading and trailing white space.
func (t Text) TrimSpace() Text {
	return Text{strings.TrimSpace(t.s)}
}

// Truncate returns t cut to at most max characters. A text that was cut ends
// in "..." when max leaves room for it. It counts characters, not bytes, and
// never cuts inside one.
func (t Text) Truncate(max int) Text {
	const dots = "..."
	if max <= 0 {
		return Text{}
	}
	if utf8.RuneCountInString(t.s) <= max {
		return t
	}
	keep, tail := max, ""
	if max > len(dots) {
		keep, tail = max-len(dots), dots
	}
	end := 0
	for i := range t.s {
		if keep == 0 {
			end = i
			break
		}
		keep--
	}
	return Text{t.s[:end] + tail}
}

const (
	escByte = 0x1b
	belByte = 0x07

	nonJoiner = 0x200c  // zero width non-joiner
	joiner    = 0x200d  // zero width joiner
	blackFlag = 0x1f3f4 // starts an emoji flag spelled with tag characters
	tagFirst  = 0xe0020
	tagLast   = 0xe007f // the cancel tag, which ends a flag
)

// clean is the sanitiser. oneLine selects Line's treatment of newline, tab
// and carriage return.
func clean(s string, oneLine bool, policy Policy) (string, Report) {
	// Most text needs nothing done: printable ASCII, and for Clean newline
	// and tab.
	i := 0
	for i < len(s) {
		c := s[i]
		if (c < 0x20 || c >= 0x7f) && (oneLine || (c != '\n' && c != '\t')) {
			break
		}
		i++
	}
	if i == len(s) {
		return s, Report{}
	}

	var rep Report
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:i])

	// prev is the last character written, 0 at the start of the text and
	// of every line: a joiner is judged by what it follows. inFlag is set
	// from a black flag to the end of the tag characters after it.
	var prev rune
	if i > 0 {
		prev = rune(s[i-1])
	}
	inFlag := false

	for i < len(s) {
		c := s[i]
		if c < utf8.RuneSelf {
			inFlag = false
		}
		switch {
		case c == escByte:
			i = skipEscape(s, i+1, &rep)
		case c == '\n' || c == '\t':
			if oneLine {
				c = ' '
			}
			b.WriteByte(c)
			prev = 0
			i++
		case c == '\r':
			i++
			if i < len(s) && s[i] == '\n' {
				continue // CR LF: the LF is written next
			}
			if oneLine {
				b.WriteByte(' ')
			} else {
				b.WriteByte('\n')
			}
			prev = 0
		case c < 0x20 || c == 0x7f:
			rep.Controls++
			i++
		case c < utf8.RuneSelf:
			b.WriteByte(c)
			prev = rune(c)
			i++
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			wasInFlag := inFlag
			inFlag = false
			switch {
			case r == utf8.RuneError && size == 1:
				rep.Invalid++
				b.WriteRune(utf8.RuneError)
				prev = utf8.RuneError
				i++
			case r >= 0x80 && r <= 0x9f:
				i = skipC1(s, i+size, r, &rep)
			case isBidiControl(r):
				rep.Bidi++
				prev = mark(&b, r, policy, prev)
				i += size
			case isInvisible(r, prev, wasInFlag):
				rep.Invisible++
				prev = mark(&b, r, policy, prev)
				i += size
			default:
				b.WriteString(s[i : i+size])
				// A combining mark belongs to the character before
				// it: a joiner after it is judged by that character.
				if !unicode.IsMark(r) {
					prev = r
				}
				inFlag = r == blackFlag || (wasInFlag && r >= tagFirst && r < tagLast)
				i += size
			}
		}
	}
	return b.String(), rep
}

// mark writes the marker for r under the Mark policy and nothing under Drop.
// It returns the last character written.
func mark(b *strings.Builder, r rune, policy Policy, prev rune) rune {
	if policy == Drop {
		return prev
	}
	fmt.Fprintf(b, "<U+%04X>", r)
	return '>'
}

// isBidiControl reports whether r is one of the twelve bidirectional
// controls: U+061C, U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069.
func isBidiControl(r rune) bool {
	return unicode.Is(unicode.Bidi_Control, r)
}

// isInvisible reports whether r is an invisible format character where it
// stands. prev is the character before it, 0 at the start of a line, and
// inFlag says whether r follows a black flag and its tag characters.
func isInvisible(r, prev rune, inFlag bool) bool {
	switch {
	case r == nonJoiner || r == joiner:
		return !takesJoiner(prev)
	case r >= tagFirst && r <= tagLast:
		return !inFlag
	case unicode.Is(unicode.Variation_Selector, r):
		return false
	case unicode.Is(unicode.Cf, r):
		// Format characters that draw: the Arabic and other number signs
		// that enclose the digits after them, and the Egyptian
		// hieroglyph format controls, which stack signs.
		return !unicode.Is(unicode.Prepended_Concatenation_Mark, r) && !(r >= 0x13430 && r <= 0x1345f)
	}
	return unicode.Is(unicode.Other_Default_Ignorable_Code_Point, r)
}

// takesJoiner reports whether a zero width joiner or non-joiner after prev
// can be doing its job. It cannot at the start of a line, after ASCII, after
// a Latin, Greek or Cyrillic letter or after white space; after anything else
// (an emoji, a variation selector, a letter of a script that joins) it is
// left alone.
func takesJoiner(prev rune) bool {
	if prev < utf8.RuneSelf || prev == utf8.RuneError || unicode.IsSpace(prev) {
		return false
	}
	return !unicode.In(prev, unicode.Latin, unicode.Greek, unicode.Cyrillic)
}

// skipEscape is called with i just past an ESC. It returns the index of the
// first byte that is not part of the escape sequence, and counts what the
// escape was in rep.
func skipEscape(s string, i int, rep *Report) int {
	if i >= len(s) {
		rep.Controls++
		return i
	}
	switch c := s[i]; {
	case c == '[':
		rep.Sequences++
		return skipCSI(s, i+1)
	case c == ']':
		return skipString(s, i+1, true, rep)
	case c == 'P' || c == 'X' || c == '^' || c == '_':
		return skipString(s, i+1, false, rep)
	case c >= 0x20 && c <= 0x2f:
		// ESC, intermediate bytes, one final byte: charset selection and
		// the like.
		rep.Sequences++
		for i < len(s) && s[i] >= 0x20 && s[i] <= 0x2f {
			i++
		}
		if i < len(s) && s[i] >= 0x30 && s[i] <= 0x7e {
			i++
		}
		return i
	case c >= 0x30 && c <= 0x7e:
		rep.Sequences++
		return i + 1 // a two-byte sequence: ESC c, ESC 7, a stray ST
	default:
		rep.Controls++
		return i // ESC before something that cannot continue it
	}
}

// skipC1 is called with i just past the C1 control r, which was encoded as a
// UTF-8 character.
func skipC1(s string, i int, r rune, rep *Report) int {
	switch r {
	case 0x9b: // CSI
		rep.Sequences++
		return skipCSI(s, i)
	case 0x9d: // OSC
		return skipString(s, i, true, rep)
	case 0x90, 0x98, 0x9e, 0x9f: // DCS, SOS, PM, APC
		return skipString(s, i, false, rep)
	default:
		rep.Controls++
		return i
	}
}

// skipCSI is called with i just past a CSI introducer. A sequence is
// parameter bytes, intermediate bytes and one final byte; one that breaks off
// ends where it broke, and the byte that broke it is ordinary input.
func skipCSI(s string, i int) int {
	for i < len(s) && s[i] >= 0x30 && s[i] <= 0x3f {
		i++
	}
	for i < len(s) && s[i] >= 0x20 && s[i] <= 0x2f {
		i++
	}
	if i < len(s) && s[i] >= 0x40 && s[i] <= 0x7e {
		i++
	}
	return i
}

// skipString is called with i just past the introducer of an OSC, DCS, APC,
// PM or SOS string. It returns the index after the terminator, or where an
// unterminated string stops: at the end of the line, at the end of s, or at
// an ESC that starts another sequence, as it would in a terminal. It counts
// the string in rep as a sequence or as unterminated.
func skipString(s string, i int, osc bool, rep *Report) int {
	for i < len(s) {
		switch c := s[i]; {
		case c == escByte:
			if i+1 < len(s) && s[i+1] == '\\' {
				rep.Sequences++
				return i + 2
			}
			rep.Unterminated++
			return i
		case c == belByte && osc:
			rep.Sequences++
			return i + 1
		case c == '\n' || c == '\r':
			rep.Unterminated++
			return i
		case c == 0xc2 && i+1 < len(s) && s[i+1] == 0x9c: // U+009C, ST
			rep.Sequences++
			return i + 2
		}
		i++
	}
	rep.Unterminated++
	return i
}
