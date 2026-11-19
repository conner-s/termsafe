package termsafe

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// u is the character with code point r. As in the other tests, no invisible
// character is written into this file.
func u(r rune) string { return string(r) }

const (
	vs1Str      = string(rune(0xFE00))  // variation selector 1
	vs2Str      = string(rune(0xFE01))  // variation selector 2
	vs3Str      = string(rune(0xFE02))  // variation selector 3
	vs4Str      = string(rune(0xFE03))  // variation selector 4
	vs8Str      = string(rune(0xFE07))  // variation selector 8, which no sequence uses
	vs15        = string(rune(0xFE0E))  // variation selector 15: text presentation
	vs17        = string(rune(0xE0100)) // variation selector 17, the first for ideographs
	vs256       = string(rune(0xE01EF)) // variation selector 256, the last
	fvs1        = string(rune(0x180B))  // Mongolian free variation selector one
	lineSep     = string(rune(0x2028))  // line separator
	paraSep     = string(rune(0x2029))  // paragraph separator
	heart       = string(rune(0x2764))  // heavy black heart
	hanKatsu    = string(rune(0x845B))  // a Han character with registered ideographic variants
	mongolianA  = string(rune(0x1820))  // Mongolian letter a
	hieroglyph  = string(rune(0x13091)) // an Egyptian hieroglyph with rotated variants
	combAcute   = string(rune(0x0301))  // combining acute accent
	whiteFlag   = string(rune(0x1F3F3)) // waving white flag
	rainbow     = string(rune(0x1F308))
	eye         = string(rune(0x1F441))
	speechLeft  = string(rune(0x1F5E8)) // left speech bubble
	infoSource  = string(rune(0x2139))  // information source, a letter that is an emoji
	wavyDash    = string(rune(0x3030))
	leftRight   = string(rune(0x2194)) // left right arrow
	copyright   = string(rune(0x00A9))
	ideoComma   = string(rune(0x3001)) // ideographic comma
	scriptCapA  = string(rune(0x1D49C))
	scriptCapH  = string(rune(0x210B))
	intersect   = string(rune(0x2229)) // intersection, with a serif variant
	fullZero    = string(rune(0xFF10)) // fullwidth digit zero
	myanmarKa   = string(rune(0x1000))
	phagsPaKa   = string(rune(0xA856))
	manichaean  = string(rune(0x10AC5))
	hanCompat   = string(rune(0x349E)) // a Han character with a standardized variant
	leftQuote   = string(rune(0x2018))
	doubleBang  = string(rune(0x203C))
	partAltMark = string(rune(0x303D))
	hieroFormat = string(rune(0x13430)) // Egyptian hieroglyph vertical joiner, a format character
)

// TestSelectorsOnTheirBases: a variation selector directly after a character
// it can modify is legitimate text. It comes through byte for byte under both
// policies, with nothing reported.
func TestSelectorsOnTheirBases(t *testing.T) {
	cases := []struct{ name, text string }{
		{"emoji presentation", heart + vs16},
		{"text presentation", leftRight + vs15 + " " + copyright + vs15},
		{"keycaps", "1" + vs16 + u(0x20E3) + " #" + vs16 + u(0x20E3) + " *" + vs15},
		{"emoji joined after a selector", whiteFlag + vs16 + zwj + rainbow},
		{"two selectors in one emoji, each on its own base", eye + vs16 + zwj + speechLeft + vs16},
		{"emoji that are not symbols", infoSource + vs16 + wavyDash + vs16 + doubleBang + vs16 + partAltMark + vs15},
		{"ideographic variation sequences", hanKatsu + vs17 + hanKatsu + vs256},
		{"standardized variants of Han characters", hanCompat + vs1Str + hanCompat + vs3Str},
		{"Mongolian free variation selector", mongolianA + fvs1 + mongolianA + u(0x180F)},
		{"Egyptian hieroglyph turned", hieroglyph + vs1Str + hieroglyph + u(0xFE06)},
		{"zero with a stroke", "0" + vs1Str + " " + fullZero + vs1Str},
		{"mathematical operator", intersect + vs1Str},
		{"script capitals", scriptCapA + vs1Str + scriptCapA + vs2Str + scriptCapH + vs2Str},
		{"East Asian punctuation", ideoComma + vs1Str + ideoComma + vs2Str},
		{"quotation marks", leftQuote + vs1Str + leftQuote + vs2Str + leftQuote + vs3Str},
		{"Myanmar, Phags-pa, Manichaean", myanmarKa + vs1Str + phagsPaKa + vs1Str + manichaean + vs1Str},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for name, policy := range map[string]Policy{"Drop": Drop, "Mark": Mark} {
				for shape, clean := range map[string]func(string) (Text, Report){"Clean": policy.CleanReport, "Line": policy.LineReport} {
					text, rep := clean(tc.text)
					if text.String() != tc.text {
						t.Errorf("%s.%s(%+q) = %+q, want it untouched", name, shape, tc.text, text.String())
					}
					if rep.Changed() {
						t.Errorf("%s.%s reported %+v for %+q, want nothing", name, shape, rep, tc.text)
					}
				}
			}
			checkInvariants(t, tc.text)
		})
	}
}

// strayCases: selectors that select nothing. Each is an invisible character
// like any other: dropped or marked, and counted.
var strayCases = []struct {
	name     string
	in       string
	wantDrop string
	wantMark string
	want     Report
}{
	{
		name:     "after a Latin letter",
		in:       "a" + vs16 + "b",
		wantDrop: "ab",
		wantMark: "a<U+FE0F>b",
		want:     Report{Invisible: 1},
	},
	{
		name:     "a run hiding bytes after an emoji: the first selector is the emoji's, the rest are not",
		in:       heart + vs16 + vs1Str + vs17 + vs256 + vs16 + " ok",
		wantDrop: heart + vs16 + " ok",
		wantMark: heart + vs16 + "<U+FE00><U+E0100><U+E01EF><U+FE0F> ok",
		want:     Report{Invisible: 4},
	},
	{
		name:     "a run after a letter",
		in:       "x" + vs17 + u(0xE0148) + u(0xE0165) + u(0xE016C) + "y",
		wantDrop: "xy",
		wantMark: "x<U+E0100><U+E0148><U+E0165><U+E016C>y",
		want:     Report{Invisible: 4},
	},
	{
		name:     "the same selector twice",
		in:       heart + vs16 + vs16,
		wantDrop: heart + vs16,
		wantMark: heart + vs16 + "<U+FE0F>",
		want:     Report{Invisible: 1},
	},
	{
		name:     "at the start of the text, after a space, at the start of a line",
		in:       vs16 + "a " + vs15 + "b\n" + vs17 + "c",
		wantDrop: "a b\nc",
		wantMark: "<U+FE0F>a <U+FE0E>b\n<U+E0100>c",
		want:     Report{Invisible: 3},
	},
	{
		name:     "an ideographic selector on what is not an ideograph",
		in:       heart + vs17 + "a" + vs256,
		wantDrop: heart + "a",
		wantMark: heart + "<U+E0100>a<U+E01EF>",
		want:     Report{Invisible: 2},
	},
	{
		name:     "an emoji selector on an ideograph, and VS4 on one",
		in:       hanKatsu + vs16 + hanKatsu + vs4Str,
		wantDrop: hanKatsu + hanKatsu,
		wantMark: hanKatsu + "<U+FE0F>" + hanKatsu + "<U+FE03>",
		want:     Report{Invisible: 2},
	},
	{
		name:     "VS8, which no sequence uses, on a hieroglyph, and VS4 on a hieroglyph's format character",
		in:       hieroglyph + vs8Str + hieroFormat + vs4Str,
		wantDrop: hieroglyph + hieroFormat,
		wantMark: hieroglyph + "<U+FE07>" + hieroFormat + "<U+FE03>",
		want:     Report{Invisible: 2},
	},
	{
		name:     "a Mongolian selector on a Latin letter and on a digit",
		in:       "a" + fvs1 + u(0x1810) + fvs1,
		wantDrop: "a" + u(0x1810),
		wantMark: "a<U+180B>" + u(0x1810) + "<U+180B>",
		want:     Report{Invisible: 2},
	},
	{
		name:     "VS1 on a symbol that is not mathematical, on a digit that is not zero, on a plus sign",
		in:       heart + vs1Str + "1" + vs1Str + "+" + vs16,
		wantDrop: heart + "1+",
		wantMark: heart + "<U+FE00>1<U+FE00>+<U+FE0F>",
		want:     Report{Invisible: 3},
	},
	{
		name:     "VS2 and VS3 where only VS1 is defined",
		in:       intersect + vs2Str + myanmarKa + vs2Str + ideoComma + vs3Str + scriptCapA + vs3Str,
		wantDrop: intersect + myanmarKa + ideoComma + scriptCapA,
		wantMark: intersect + "<U+FE01>" + myanmarKa + "<U+FE01>" + ideoComma + "<U+FE02>" + scriptCapA + "<U+FE02>",
		want:     Report{Invisible: 4},
	},
	{
		name:     "after a combining mark: the selector must stand directly after its base",
		in:       heart + combAcute + vs16,
		wantDrop: heart + combAcute,
		wantMark: heart + combAcute + "<U+FE0F>",
		want:     Report{Invisible: 1},
	},
	{
		name:     "with control sequences around it",
		in:       "\x1b[1m" + vs16 + "a\x1b[0m" + vs17,
		wantDrop: "a",
		wantMark: "<U+FE0F>a<U+E0100>",
		want:     Report{Sequences: 2, Invisible: 2},
	},
}

func TestStraySelectors(t *testing.T) {
	for _, tc := range strayCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, p := range []struct {
				name   string
				policy Policy
				want   string
			}{{"Drop", Drop, tc.wantDrop}, {"Mark", Mark, tc.wantMark}} {
				text, rep := p.policy.CleanReport(tc.in)
				if text.String() != p.want {
					t.Errorf("%s.Clean(%+q)\n got %+q\nwant %+q", p.name, tc.in, text.String(), p.want)
				}
				if rep != tc.want {
					t.Errorf("%s reported %+v, want %+v", p.name, rep, tc.want)
				}
				line, lineRep := p.policy.LineReport(tc.in)
				wantLine := strings.ReplaceAll(p.want, "\n", " ")
				if line.String() != wantLine || lineRep != tc.want {
					t.Errorf("%s.Line(%+q) = %+q with %+v, want %+q with %+v", p.name, tc.in, line.String(), lineRep, wantLine, tc.want)
				}
			}
			checkInvariants(t, tc.in)
		})
	}
}

// TestSelectorAfterSomethingRemoved: a selector is judged by what stands
// before it in the result. Under Drop the character between is gone and the
// selector meets its base; under Mark a marker stands between them.
func TestSelectorAfterSomethingRemoved(t *testing.T) {
	in := heart + zwsp + vs16
	if got, want := Drop.Line(in).String(), heart+vs16; got != want {
		t.Errorf("Drop.Line(%+q) = %+q, want %+q", in, got, want)
	}
	if got, want := Mark.Line(in).String(), heart+"<U+200B><U+FE0F>"; got != want {
		t.Errorf("Mark.Line(%+q) = %+q, want %+q", in, got, want)
	}
	checkInvariants(t, in)
}

// variationSequences reads the base and selector of every sequence in one of
// Unicode's data files: lines of "base selector ; description".
func variationSequences(t *testing.T, path string) [][2]rune {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var seqs [][2]rune
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		line, _, _ := strings.Cut(lines.Text(), "#")
		codes, _, _ := strings.Cut(line, ";")
		fields := strings.Fields(codes)
		if len(fields) != 2 {
			continue
		}
		base, err1 := strconv.ParseUint(fields[0], 16, 32)
		sel, err2 := strconv.ParseUint(fields[1], 16, 32)
		if err1 != nil || err2 != nil {
			t.Fatalf("%s: cannot read %q", path, lines.Text())
		}
		seqs = append(seqs, [2]rune{rune(base), rune(sel)})
	}
	if err := lines.Err(); err != nil {
		t.Fatal(err)
	}
	return seqs
}

// TestSelectorsAgainstUnicodeData holds the rule to Unicode's own lists: every
// standardized variation sequence and every emoji variation sequence of
// Unicode 17.0 comes through untouched and unreported.
//
// A sequence whose base this Go release does not know yet is left out and
// counted: built with Go 1.26, whose tables are Unicode 15.0, the package
// removes a selector after a character that Unicode added later. With tables
// of 17.0 or newer nothing may be left out.
func TestSelectorsAgainstUnicodeData(t *testing.T) {
	for _, file := range []struct {
		path string
		min  int // the file has at least this many sequences
	}{
		{"testdata/unicode/StandardizedVariants.txt", 1300},
		{"testdata/unicode/emoji-variation-sequences.txt", 700},
	} {
		seqs := variationSequences(t, file.path)
		if len(seqs) < file.min {
			t.Fatalf("%s: read %d sequences, want at least %d", file.path, len(seqs), file.min)
		}
		unknown := 0
		for _, seq := range seqs {
			base, sel := seq[0], seq[1]
			if !unicode.In(base, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S) {
				unknown++
				continue
			}
			if !unicode.Is(unicode.Variation_Selector, sel) {
				t.Errorf("%s: %U %U: the second is not a variation selector", file.path, base, sel)
				continue
			}
			in := "a " + string(base) + string(sel) + " b"
			for name, policy := range map[string]Policy{"Drop": Drop, "Mark": Mark} {
				if text, rep := policy.LineReport(in); text.String() != in || rep.Changed() {
					t.Errorf("%s: %U %U is a sequence Unicode defines, but %s gives %+q and reports %+v", file.path, base, sel, name, text.String(), rep)
				}
			}
		}
		t.Logf("%s: %d sequences, %d with a base unknown to Unicode %s", file.path, len(seqs), unknown, unicode.Version)
		if unknown > 0 && !olderThan(unicode.Version, "17.0.0") {
			t.Errorf("%s: %d bases are unknown to Unicode %s, which should know all of 17.0", file.path, unknown, unicode.Version)
		}
	}
}

// olderThan compares two Unicode versions by their major number.
func olderThan(version, than string) bool {
	major := func(v string) int {
		head, _, _ := strings.Cut(v, ".")
		n, _ := strconv.Atoi(head)
		return n
	}
	return major(version) < major(than)
}

// TestUnicodeVersion records which Unicode version the tests ran under. What
// counts as a letter, a symbol, an ideograph or a format character comes from
// the standard library, so the same input can clean differently under another
// Go release. Run it with -v to see the line.
func TestUnicodeVersion(t *testing.T) {
	if unicode.Version == "" {
		t.Fatal("the standard library names no Unicode version")
	}
	t.Logf("termsafe is using the tables of Unicode %s", unicode.Version)
}

func TestLineSeparators(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantClean string
		wantLine  string
	}{
		{"line separator", "a" + lineSep + "b", "a\nb", "a b"},
		{"paragraph separator", "a" + paraSep + "b", "a\nb", "a b"},
		{"both, and a newline", "title" + lineSep + paraSep + "\nnext", "title\n\n\nnext", "title   next"},
		{"alone", lineSep, "\n", " "},
		{"inside a control string it goes with the payload", "a\x1b]0;x" + lineSep + "y\x07b", "ab", "ab"},
		{"it does not end an unterminated control string", "a\x1bPq" + lineSep + "y\nb", "a\nb", "a b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for name, policy := range map[string]Policy{"Drop": Drop, "Mark": Mark} {
				if got := policy.Clean(tc.in).String(); got != tc.wantClean {
					t.Errorf("%s.Clean(%+q) = %+q, want %+q", name, tc.in, got, tc.wantClean)
				}
				if got := policy.Line(tc.in).String(); got != tc.wantLine {
					t.Errorf("%s.Line(%+q) = %+q, want %+q", name, tc.in, got, tc.wantLine)
				}
			}
			checkInvariants(t, tc.in)
		})
	}

	// Like a carriage return made a newline, it is white space and not
	// counted.
	if _, rep := Mark.CleanReport("a" + lineSep + "b" + paraSep); rep.Changed() {
		t.Errorf("a line separator was reported: %+v", rep)
	}
	// It starts a line: a joiner or a selector after it follows nothing.
	if got := Drop.Clean(heart + lineSep + zwj + vs16 + "x").String(); got != heart+"\nx" {
		t.Errorf("after a line separator: %+q", got)
	}
}
