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
//   - Call [CheckTerminal] once at start and pass every Text through
//     [Terminal.Fit] on its way to the screen. What follows holds on a
//     terminal that decodes UTF-8; on any other, only the ASCII form that Fit
//     then returns is safe. See "Terminals that are not UTF-8" below.
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
//   - A variation selector that selects nothing: one that does not stand
//     directly after a character it can modify, or a second one in a row.
//   - A line break other than a newline.
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
// separates the states of a line it redraws. The line separator U+2028 and
// the paragraph separator U+2029 are line breaks too: Clean makes each a
// newline and Line a space, so a newline is the only line break a Text holds
// and a Line holds none. Like a carriage return they are white space, and the
// Report does not count them. Inside a control string they are payload: only
// a newline or a carriage return ends an unterminated string, as in a
// terminal.
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
//   - Variation selectors (U+FE00 to U+FE0F, U+E0100 to U+E01EF and the
//     Mongolian U+180B to U+180D and U+180F) that select nothing. A selector
//     draws nothing itself, and a run of them after any character can carry
//     hidden bytes the way tag characters can. One selector directly after a
//     character it can modify is kept; every other one counts as invisible.
//
// These are left alone, because legitimate text needs them: a joiner after
// any other character (the zero width joiner builds emoji such as a family
// or a profession, and Arabic, Persian and Indic scripts use both joiners);
// a variation selector on its base, as described next; tag characters
// that follow U+1F3F4, which spell the flags of England, Scotland and Wales;
// the Arabic number signs, which are format characters that do draw. Text in
// a right-to-left script is ordinary text and is never touched.
//
// # Variation selectors
//
// A selector is kept when it stands directly after one of these, with no
// combining mark and no other selector in between:
//
//   - U+FE0E and U+FE0F (text and emoji presentation): a symbol of the
//     categories So and Sm outside ASCII; the keycap bases #, * and 0 to 9;
//     and U+203C, U+2049, U+2139, U+3030 and U+303D, the five emoji that are
//     not symbols.
//   - U+E0100 to U+E01EF (ideographic variation sequences): a unified
//     ideograph.
//   - U+FE00 to U+FE02: a unified ideograph, and the quotation marks U+2018,
//     U+2019, U+201C and U+201D.
//   - U+FE00 and U+FE01: the ideographic and fullwidth punctuation U+3001,
//     U+3002, U+FF01, U+FF0C, U+FF0E, U+FF1A, U+FF1B and U+FF1F, and the
//     mathematical script capitals.
//   - U+FE00 alone: a mathematical symbol (Sm) outside ASCII, the digit zero
//     and the fullwidth zero, and a character of the Myanmar, Phags-pa or
//     Manichaean script.
//   - U+FE00 to U+FE06: an Egyptian hieroglyph.
//   - The Mongolian selectors: a Mongolian letter.
//
// U+FE07 to U+FE0D are in no sequence and are never kept.
//
// The standard library knows the category and script of a character but has
// no list of the sequences Unicode defines, so the rule goes by class. It
// keeps every standardized variation sequence, every emoji variation sequence
// and every sequence registered in the Ideographic Variation Database, as of
// Unicode 17.0. The tests hold it to the kinds of sequence in those lists,
// and to every sequence when they are given Unicode's files. It also keeps
// sequences Unicode does not define: any of the 240 ideographic selectors on
// any of about 100,000 ideographs, where some 39,500 pairs are registered;
// either presentation selector on any of about 8,400 symbols, where 371 have
// both presentations; and likewise for the smaller classes. What is left of
// the hidden channel is therefore one selector on each character that could
// carry one: nothing in ASCII, Latin, Greek or Cyrillic text, one of two on a
// symbol, and up to a byte for each ideograph in Chinese, Japanese or Korean
// text. Closing that needs the lists themselves.
//
// A selector is judged by what stands before it in the result. Under Drop,
// when an invisible character between a base and its selector is removed, the
// selector meets its base and stays; under Mark the marker stands between
// them and the selector is marked too.
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
// # Terminals that are not UTF-8
//
// Everything above is about characters, and holds when the terminal decodes
// the bytes it is sent as UTF-8. A terminal in an 8-bit mode does not: it
// reads each of the bytes 0x80 to 0x9F as a C1 control, and ordinary
// characters hold those bytes in their UTF-8 encoding. U+00DB is C3 9B, and
// 9B is CSI. No choice of characters is safe there, only ASCII.
//
// [CheckTerminal] says what the environment tells about the terminal: UTF-8
// as far as it says, not UTF-8, or cannot tell. [Text.ASCII] is the form for
// the last two, with every character outside ASCII shown as its code point,
// and [Terminal.Fit] chooses between a Text and its ASCII form. The check
// reads the locale and, on Windows, what kind of handle the output is; it
// cannot see how the terminal emulator itself is set. [Terminal.Probe] can:
// it asks the terminal, for a program that owns it and chooses to. A program
// that draws a Text without asking either is assuming UTF-8.
//
// # Unicode versions
//
// Which characters are letters, symbols, ideographs, format characters and
// members of a script comes from the unicode package, so the tables are those
// of the Go release a program is built with: Unicode 15.0 in Go 1.26, Unicode
// 17.0 in Go 1.27. The toolchain of the program decides, not this module.
//
// Between those two versions the sets of bidirectional controls, format
// characters, default-ignorable characters and variation selectors are the
// same, and no character of 15.0 changed in any property used here. What
// differs is the 10,615 characters added since. Built with Go 1.26 the
// package does not know them, so:
//
//   - a variation selector after one of them (4,938 new ideographs, 3,995
//     new hieroglyphs, 846 new symbols) is removed or marked, where Go 1.27
//     keeps it;
//   - [Text.MixedScripts] does not count them as letters (9,568 are, 11 of
//     them Latin and 2 Cyrillic, the rest in other scripts, 11 of which are
//     new), so a word they stand in is split there and is not reported;
//   - a joiner after one of the new Latin or Cyrillic letters is kept, where
//     Go 1.27 removes it.
//
// A newer Go only ever knows more characters, and none loses a property it
// had. What that does to one input goes both ways, as the list shows: a
// selector is kept that was removed, a joiner is removed that was kept, a
// word is reported that was not.
//
// The package depends on the standard library only and nothing in it is
// specific to one program. PORTING.md, beside this file, says how to put it
// into one.
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

// Line returns s made safe to draw as one line: newlines, tabs and the other
// line breaks become spaces. Use it for a log line, a status, a file path or
// a URL.
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
// or a line or paragraph separator made a newline, or Line's newlines and
// tabs made spaces, are not counted.
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
	// whether the policy dropped or marked them. A variation selector
	// that selects nothing is one of them.
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

// ASCII returns t with every character outside ASCII shown by its code point,
// such as <U+65E5> for a Han character: the same marker the [Mark] policy
// writes. The result holds printable ASCII and, where t had them, newlines and
// tabs, and no other byte.
//
// Use it for a terminal that cannot be taken to decode UTF-8; [CheckTerminal]
// says whether that is so. In an 8-bit terminal the bytes 0x80 to 0x9F are C1
// controls, and ordinary characters hold those bytes in their UTF-8 encoding
// (U+00DB is C3 9B, and 9B is CSI), so there only ASCII is safe to draw. A
// text that is ASCII already is returned as it is.
//
// The markers make the text longer, eight characters or more for one. Call
// [Text.MixedScripts] before ASCII: afterwards there are no letters of other
// scripts left to find.
func (t Text) ASCII() Text {
	i := 0
	for i < len(t.s) && t.s[i] < utf8.RuneSelf {
		i++
	}
	if i == len(t.s) {
		return t
	}
	var b strings.Builder
	b.Grow(len(t.s) + 16)
	b.WriteString(t.s[:i])
	for _, r := range t.s[i:] {
		if r < utf8.RuneSelf {
			b.WriteByte(byte(r))
			continue
		}
		writeMarker(&b, r)
	}
	return Text{b.String()}
}

const (
	escByte = 0x1b
	belByte = 0x07

	nonJoiner = 0x200c  // zero width non-joiner
	joiner    = 0x200d  // zero width joiner
	blackFlag = 0x1f3f4 // starts an emoji flag spelled with tag characters
	tagFirst  = 0xe0020
	tagLast   = 0xe007f // the cancel tag, which ends a flag

	lineSeparator      = 0x2028
	paragraphSeparator = 0x2029

	// The variation selectors that have a use, by the names Unicode gives
	// them. VS8 to VS14 (U+FE07 to U+FE0D) have none.
	vs1              = 0xfe00 // the first of VS1 to VS16
	vs2              = 0xfe01
	vs3              = 0xfe02
	vs7              = 0xfe06  // the last that turns or mirrors a hieroglyph
	textSelector     = 0xfe0e  // VS15, text presentation
	emojiSelector    = 0xfe0f  // VS16, emoji presentation
	ideographicFirst = 0xe0100 // VS17, the first for ideographs
	mongolianFirst   = 0x180b  // the Mongolian free variation selectors
	mongolianLast    = 0x180f
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
	// of every line: a joiner is judged by what it follows. last is the
	// same, except that a combining mark counts too: a variation selector
	// is judged by what stands directly before it. inFlag is set from a
	// black flag to the end of the tag characters after it.
	var prev, last rune
	if i > 0 {
		prev = rune(s[i-1])
		last = prev
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
			prev, last = 0, 0
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
			prev, last = 0, 0
		case c < 0x20 || c == 0x7f:
			rep.Controls++
			i++
		case c < utf8.RuneSelf:
			b.WriteByte(c)
			prev, last = rune(c), rune(c)
			i++
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			wasInFlag := inFlag
			inFlag = false
			switch {
			case r == utf8.RuneError && size == 1:
				rep.Invalid++
				b.WriteRune(utf8.RuneError)
				prev, last = utf8.RuneError, utf8.RuneError
				i++
			case r >= 0x80 && r <= 0x9f:
				i = skipC1(s, i+size, r, &rep)
			case r == lineSeparator || r == paragraphSeparator:
				// A line break like the others: a newline, or a space
				// where the text must stay one line.
				if oneLine {
					b.WriteByte(' ')
				} else {
					b.WriteByte('\n')
				}
				prev, last = 0, 0
				i += size
			case isBidiControl(r):
				rep.Bidi++
				prev, last = mark(&b, r, policy, prev, last)
				i += size
			case isInvisible(r, prev, last, wasInFlag):
				rep.Invisible++
				prev, last = mark(&b, r, policy, prev, last)
				i += size
			default:
				b.WriteString(s[i : i+size])
				// A combining mark belongs to the character before
				// it: a joiner after it is judged by that character.
				if !unicode.IsMark(r) {
					prev = r
				}
				last = r
				inFlag = r == blackFlag || (wasInFlag && r >= tagFirst && r < tagLast)
				i += size
			}
		}
	}
	return b.String(), rep
}

// mark writes the marker for r under the Mark policy and nothing under Drop.
// It returns what prev and last are afterwards: the marker's last character,
// or what they were.
func mark(b *strings.Builder, r rune, policy Policy, prev, last rune) (rune, rune) {
	if policy == Drop {
		return prev, last
	}
	writeMarker(b, r)
	return '>', '>'
}

// writeMarker writes the visible form of r, such as <U+202E>.
func writeMarker(b *strings.Builder, r rune) {
	fmt.Fprintf(b, "<U+%04X>", r)
}

// isBidiControl reports whether r is one of the twelve bidirectional
// controls: U+061C, U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069.
func isBidiControl(r rune) bool {
	return unicode.Is(unicode.Bidi_Control, r)
}

// isInvisible reports whether r is an invisible format character where it
// stands. prev is the character before it, 0 at the start of a line; last is
// what stands directly before it, a combining mark included; inFlag says
// whether r follows a black flag and its tag characters.
func isInvisible(r, prev, last rune, inFlag bool) bool {
	switch {
	case r == nonJoiner || r == joiner:
		return !takesJoiner(prev)
	case r >= tagFirst && r <= tagLast:
		return !inFlag
	case unicode.Is(unicode.Variation_Selector, r):
		return !takesSelector(last, r)
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

// takesSelector reports whether the variation selector sel, directly after
// base, can be one of the sequences Unicode defines: the emoji and text
// presentation sequences, the standardized variation sequences, the
// ideographic variation sequences and the Mongolian ones. The standard
// library has no list of the sequences, so each selector is matched to the
// class of character it is defined for; the package comment says where that
// is wider than the list. A selector is never a base, so of two in a row the
// second fails here.
func takesSelector(base, sel rune) bool {
	switch {
	case base < utf8.RuneSelf:
		// The keycap bases take either presentation; a zero takes VS1, the
		// form with a stroke through it.
		if sel == textSelector || sel == emojiSelector {
			return base == '#' || base == '*' || (base >= '0' && base <= '9')
		}
		return sel == vs1 && base == '0'
	case sel >= mongolianFirst && sel <= mongolianLast:
		return unicode.Is(unicode.Mongolian, base) && unicode.IsLetter(base)
	case sel >= ideographicFirst:
		return unicode.Is(unicode.Unified_Ideograph, base)
	case sel == textSelector || sel == emojiSelector:
		// Every emoji that has both presentations is a symbol, apart from
		// five: the double exclamation mark, the exclamation question
		// mark, the information source, the wavy dash and the part
		// alternation mark.
		return unicode.In(base, unicode.So, unicode.Sm) ||
			base == 0x203c || base == 0x2049 || base == 0x2139 || base == 0x3030 || base == 0x303d
	case sel > vs7:
		return false
	case unicode.Is(unicode.Egyptian_Hieroglyphs, base):
		return unicode.IsLetter(base) // VS1 to VS7 turn or mirror a hieroglyph
	case sel > vs3:
		return false
	case unicode.Is(unicode.Unified_Ideograph, base):
		return true // VS1 to VS3: the forms of the compatibility ideographs
	case base == 0x2018 || base == 0x2019 || base == 0x201c || base == 0x201d:
		return true // VS1 to VS3: where a quotation mark sits in its cell
	case sel == vs3:
		return false
	case isEastAsianStop(base) || isScriptCapital(base):
		return true // VS1 and VS2
	case sel == vs2:
		return false
	}
	// VS1 alone: variants of mathematical operators, the fullwidth zero with
	// a stroke, dotted Myanmar letters, and letters of Phags-pa and
	// Manichaean.
	return unicode.Is(unicode.Sm, base) || base == 0xff10 ||
		unicode.In(base, unicode.Myanmar, unicode.Phags_Pa, unicode.Manichaean)
}

// isEastAsianStop reports whether r is one of the eight ideographic and
// fullwidth punctuation marks that VS1 and VS2 place within their cell.
func isEastAsianStop(r rune) bool {
	switch r {
	case 0x3001, 0x3002, 0xff01, 0xff0c, 0xff0e, 0xff1a, 0xff1b, 0xff1f:
		return true
	}
	return false
}

// isScriptCapital reports whether r is a mathematical script capital, which
// VS1 and VS2 set in chancery or roundhand style.
func isScriptCapital(r rune) bool {
	switch r {
	case 0x210b, 0x2110, 0x2112, 0x211b, 0x212c, 0x2130, 0x2131, 0x2133:
		return true
	}
	return r >= 0x1d49c && r <= 0x1d4b5
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
