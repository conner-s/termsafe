package termsafe

import (
	"fmt"
	"strings"
	"testing"
	"unicode"
)

// Every character these tests are about is invisible or looks like another,
// so none of them is written into this file: each is built from its code
// point and has a name.
const (
	lre  = string(rune(0x202A)) // left-to-right embedding
	rle  = string(rune(0x202B)) // right-to-left embedding
	pdf  = string(rune(0x202C)) // pop directional formatting
	lro  = string(rune(0x202D)) // left-to-right override
	rlo  = string(rune(0x202E)) // right-to-left override
	lri  = string(rune(0x2066)) // left-to-right isolate
	rli  = string(rune(0x2067)) // right-to-left isolate
	fsi  = string(rune(0x2068)) // first strong isolate
	pdi  = string(rune(0x2069)) // pop directional isolate
	lrm  = string(rune(0x200E)) // left-to-right mark
	rlm  = string(rune(0x200F)) // right-to-left mark
	alm  = string(rune(0x061C)) // Arabic letter mark
	zwsp = string(rune(0x200B)) // zero width space
	zwnj = string(rune(0x200C)) // zero width non-joiner
	zwj  = string(rune(0x200D)) // zero width joiner
	wj   = string(rune(0x2060)) // word joiner
	bom  = string(rune(0xFEFF)) // byte order mark, zero width no-break space
	shy  = string(rune(0x00AD)) // soft hyphen
	vs16 = string(rune(0xFE0F)) // variation selector 16: emoji presentation

	// Right-to-left words: shalom, salam, and Persian "mikhaham" (I want),
	// which is spelled with a zero width non-joiner inside it.
	hebrewShalom = string(rune(0x05E9)) + string(rune(0x05DC)) + string(rune(0x05D5)) + string(rune(0x05DD))
	arabicSalam  = string(rune(0x0633)) + string(rune(0x0644)) + string(rune(0x0627)) + string(rune(0x0645))
	persianWord  = string(rune(0x0645)) + string(rune(0x06CC)) + zwnj +
		string(rune(0x062E)) + string(rune(0x0648)) + string(rune(0x0627)) + string(rune(0x0647)) + string(rune(0x0645))
	// Devanagari "ksha" written with an explicit joiner after the virama.
	devanagariJoined = string(rune(0x0915)) + string(rune(0x094D)) + zwj + string(rune(0x0937))

	// Emoji that are sequences.
	womanTechnologist = string(rune(0x1F469)) + zwj + string(rune(0x1F4BB))
	family            = string(rune(0x1F468)) + zwj + string(rune(0x1F469)) + zwj + string(rune(0x1F467)) + zwj + string(rune(0x1F466))
	heartOnFire       = string(rune(0x2764)) + vs16 + zwj + string(rune(0x1F525))
	wavingDarkSkin    = string(rune(0x1F44B)) + string(rune(0x1F3FF))
	flagGermany       = string(rune(0x1F1E9)) + string(rune(0x1F1EA))
	keycapOne         = "1" + vs16 + string(rune(0x20E3))
)

// The flag of Scotland: a black flag, the tag characters "gbsct" and the
// cancel tag.
var flagScotland = string(rune(0x1F3F4)) + tags("gbsct") + string(rune(0xE007F))

// tags spells s in tag characters, the invisible twins of ASCII.
func tags(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(0xE0000 + r)
	}
	return b.String()
}

// formatCases: what each policy makes of bidirectional controls and invisible
// characters. Line gives the same as Clean for all of them.
var formatCases = []struct {
	name     string
	in       string
	wantDrop string
	wantMark string
	want     Report
}{
	// The Trojan Source shapes (CVE-2021-42574). Stored order is what a
	// compiler reads; the controls make a bidi-aware display show another.
	{
		name:     "override hiding a comment: the early return looks commented out",
		in:       "/*" + rlo + " } " + lri + "if (isAdmin)" + pdi + " " + lri + " begin admins only */",
		wantDrop: "/* } if (isAdmin)  begin admins only */",
		wantMark: "/*<U+202E> } <U+2066>if (isAdmin)<U+2069> <U+2066> begin admins only */",
		want:     Report{Bidi: 4},
	},
	{
		name:     "override reordering a line: a string literal that swallows code",
		in:       `access_level = "user` + rlo + " " + lri + "// Check if admin" + pdi + " " + lri + `"`,
		wantDrop: `access_level = "user // Check if admin "`,
		wantMark: `access_level = "user<U+202E> <U+2066>// Check if admin<U+2069> <U+2066>"`,
		want:     Report{Bidi: 4},
	},
	{
		name:     "override reversing a file name's extension",
		in:       "invoice" + rlo + "fdp.exe",
		wantDrop: "invoicefdp.exe",
		wantMark: "invoice<U+202E>fdp.exe",
		want:     Report{Bidi: 1},
	},
	{
		name:     "isolate left unterminated",
		in:       "v1.2.3 " + rli + "4.5.6",
		wantDrop: "v1.2.3 4.5.6",
		wantMark: "v1.2.3 <U+2067>4.5.6",
		want:     Report{Bidi: 1},
	},
	{
		name:     "embedding left unterminated at the end of a line",
		in:       "title " + rle + "\nnext line",
		wantDrop: "title \nnext line",
		wantMark: "title <U+202B>\nnext line",
		want:     Report{Bidi: 1},
	},
	{
		name:     "every bidirectional control",
		in:       lre + rle + pdf + lro + rlo + lri + rli + fsi + pdi + lrm + rlm + alm,
		wantDrop: "",
		wantMark: "<U+202A><U+202B><U+202C><U+202D><U+202E><U+2066><U+2067><U+2068><U+2069><U+200E><U+200F><U+061C>",
		want:     Report{Bidi: 12},
	},
	{
		name:     "a mark inside right-to-left text is still a control",
		in:       hebrewShalom + rlm + "!",
		wantDrop: hebrewShalom + "!",
		wantMark: hebrewShalom + "<U+200F>!",
		want:     Report{Bidi: 1},
	},

	// Invisible characters.
	{
		name:     "zero width space splitting a name",
		in:       "pay" + zwsp + "pal",
		wantDrop: "paypal",
		wantMark: "pay<U+200B>pal",
		want:     Report{Invisible: 1},
	},
	{
		name:     "joiner between Latin letters",
		in:       "pay" + zwj + "pal and requ" + zwnj + "ests",
		wantDrop: "paypal and requests",
		wantMark: "pay<U+200D>pal and requ<U+200C>ests",
		want:     Report{Invisible: 2},
	},
	{
		name:     "joiner at the start of the text, after a space, after a digit",
		in:       zwj + "a " + zwnj + "b 1" + zwj,
		wantDrop: "a b 1",
		wantMark: "<U+200D>a <U+200C>b 1<U+200D>",
		want:     Report{Invisible: 3},
	},
	{
		name:     "joiner after an accented Latin letter, precomposed and combining",
		in:       "caf" + string(rune(0x00E9)) + zwj + " cafe" + string(rune(0x0301)) + zwj,
		wantDrop: "caf" + string(rune(0x00E9)) + " cafe" + string(rune(0x0301)),
		wantMark: "caf" + string(rune(0x00E9)) + "<U+200D> cafe" + string(rune(0x0301)) + "<U+200D>",
		want:     Report{Invisible: 2},
	},
	{
		name:     "joiner after a Cyrillic or Greek letter",
		in:       string(rune(0x0430)) + zwj + string(rune(0x03B1)) + zwnj,
		wantDrop: string(rune(0x0430)) + string(rune(0x03B1)),
		wantMark: string(rune(0x0430)) + "<U+200D>" + string(rune(0x03B1)) + "<U+200C>",
		want:     Report{Invisible: 2},
	},
	{
		name:     "byte order mark, word joiner, soft hyphen",
		in:       bom + "name" + wj + "space" + shy + "d",
		wantDrop: "namespaced",
		wantMark: "<U+FEFF>name<U+2060>space<U+00AD>d",
		want:     Report{Invisible: 3},
	},
	{
		name:     "Hangul filler and Mongolian vowel separator",
		in:       "a" + string(rune(0x3164)) + "b" + string(rune(0x180E)) + "c",
		wantDrop: "abc",
		wantMark: "a<U+3164>b<U+180E>c",
		want:     Report{Invisible: 2},
	},
	{
		name:     "tag characters hiding a sentence after ordinary text",
		in:       "Looks fine." + tags("ignore"),
		wantDrop: "Looks fine.",
		wantMark: "Looks fine.<U+E0069><U+E0067><U+E006E><U+E006F><U+E0072><U+E0065>",
		want:     Report{Invisible: 6},
	},
	{
		name:     "tag characters after a flag has ended",
		in:       flagScotland + tags("x"),
		wantDrop: flagScotland,
		wantMark: flagScotland + "<U+E0078>",
		want:     Report{Invisible: 1},
	},
	{
		name:     "bidi and invisible with control sequences",
		in:       "\x1b[31m" + rlo + "red" + zwsp + "\x1b[0m\x07",
		wantDrop: "red",
		wantMark: "<U+202E>red<U+200B>",
		want:     Report{Sequences: 2, Controls: 1, Bidi: 1, Invisible: 1},
	},
}

func TestFormatCharacters(t *testing.T) {
	for _, tc := range formatCases {
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

// TestLegitimateTextIsUntouched: right-to-left text, the joiners where
// scripts and emoji need them, variation selectors and flags come through
// byte for byte, under both policies, with nothing reported and no word
// flagged.
func TestLegitimateTextIsUntouched(t *testing.T) {
	cases := []struct{ name, text string }{
		{"Hebrew", hebrewShalom + " " + hebrewShalom},
		{"Arabic", arabicSalam},
		{"Hebrew and Latin in one line", "greeting: " + hebrewShalom + " (shalom)"},
		{"Persian with a zero width non-joiner", persianWord},
		{"Devanagari with a zero width joiner", devanagariJoined},
		{"emoji joined by zero width joiners", womanTechnologist + " " + family},
		{"emoji with a variation selector before the joiner", heartOnFire},
		{"emoji with a skin tone", wavingDarkSkin},
		{"flag of regional indicators", flagGermany},
		{"flag spelled in tag characters", flagScotland + " and " + flagScotland},
		{"keycap", keycapOne},
		{"Arabic number sign, a format character that draws", string(rune(0x0600)) + string(rune(0x0661)) + string(rune(0x0662))},
		{"Japanese with Latin", "T" + string(rune(0x30B7)) + string(rune(0x30E3)) + string(rune(0x30C4)) + " " + string(rune(0x65E5)) + string(rune(0x672C)) + string(rune(0x8A9E))},
		{"accents and no-break space", "na" + string(rune(0x00EF)) + "ve caf" + string(rune(0x00E9)) + string(rune(0x00A0)) + "!"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for name, policy := range map[string]Policy{"Drop": Drop, "Mark": Mark} {
				text, rep := policy.CleanReport(tc.text)
				if text.String() != tc.text {
					t.Errorf("%s.Clean(%+q) = %+q, want it untouched", name, tc.text, text.String())
				}
				if rep.Changed() {
					t.Errorf("%s reported %+v for %+q, want nothing", name, rep, tc.text)
				}
				if mixed := text.MixedScripts(); len(mixed) != 0 {
					t.Errorf("MixedScripts flagged %v in %+q", mixed, tc.text)
				}
			}
			checkInvariants(t, tc.text)
		})
	}
}

// TestPoliciesDifferOnlyOnFormatCharacters: on every input of the control
// sequence table, which holds none, Mark and Drop give the same text.
func TestPoliciesDifferOnlyOnFormatCharacters(t *testing.T) {
	for _, tc := range cleanCases {
		drop, dropRep := Drop.CleanReport(tc.in)
		mark, markRep := Mark.CleanReport(tc.in)
		if drop != mark || dropRep != markRep {
			t.Errorf("%s: Drop gives %+q %+v, Mark gives %+q %+v", tc.name, drop.String(), dropRep, mark.String(), markRep)
		}
	}
}

// TestZeroPolicyMarks: a Policy nobody set must not hide anything.
func TestZeroPolicyMarks(t *testing.T) {
	var p Policy
	if got := p.Line("a" + rlo + "b").String(); got != "a<U+202E>b" {
		t.Errorf("the zero Policy gives %+q, want the control marked", got)
	}
}

// TestBidiControlsAreTheTwelve pins the set against the standard library's
// table, so a new Unicode version that adds one is noticed here.
func TestBidiControlsAreTheTwelve(t *testing.T) {
	want := map[rune]bool{}
	for _, s := range []string{lre, rle, pdf, lro, rlo, lri, rli, fsi, pdi, lrm, rlm, alm} {
		want[[]rune(s)[0]] = true
	}
	n := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if isBidiControl(r) {
			n++
			if !want[r] {
				t.Errorf("U+%04X is a bidirectional control the tests do not know", r)
			}
		}
	}
	if n != len(want) {
		t.Errorf("%d bidirectional controls, want %d", n, len(want))
	}
}

// TestNoFormatCharacterSurvives walks every code point: after cleaning under
// either policy, no bidirectional control and no invisible character is left,
// wherever it stood.
func TestNoFormatCharacterSurvives(t *testing.T) {
	for r := rune(0x80); r <= unicode.MaxRune; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		if !isBidiControl(r) && !isInvisible(r, 'a', false) {
			continue
		}
		in := "a" + string(r) + "b"
		if got := Drop.Line(in).String(); got != "ab" {
			t.Fatalf("Drop.Line(a U+%04X b) = %+q, want %q", r, got, "ab")
		}
		if got, want := Mark.Line(in).String(), fmt.Sprintf("a<U+%04X>b", r); got != want {
			t.Fatalf("Mark.Line(a U+%04X b) = %+q, want %+q", r, got, want)
		}
	}
}
