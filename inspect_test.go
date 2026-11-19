package termsafe

import (
	"reflect"
	"testing"
)

// Look-alike letters, built from their code points so that none stands in
// this file looking like the Latin letter it imitates.
const (
	cyrA       = string(rune(0x0430)) // Cyrillic a
	cyrO       = string(rune(0x043E)) // Cyrillic o
	cyrP       = string(rune(0x0440)) // Cyrillic er, looks like p
	cyrE       = string(rune(0x0435)) // Cyrillic ie, looks like e
	cyrPalochk = string(rune(0x04CF)) // Cyrillic palochka, looks like l
	grUpsilon  = string(rune(0x03C5)) // Greek upsilon, looks like u
	grOmicron  = string(rune(0x03BF)) // Greek omicron, looks like o
	grMu       = string(rune(0x03BC)) // Greek mu
	armO       = string(rune(0x0585)) // Armenian oh, looks like o
)

func TestMixedScripts(t *testing.T) {
	type finding struct {
		word    string
		start   int
		scripts []string
		odd     []rune
	}
	cases := []struct {
		name string
		text string
		want []finding
	}{
		{"ascii", "github.com/charmbracelet/bubbletea v2.1.1", nil},
		{"latin with accents", "na" + string(rune(0x00EF)) + "ve caf" + string(rune(0x00E9)), nil},
		{"whole words of different scripts side by side", "hello " + cyrP + cyrA + " " + grMu + grOmicron, nil},

		{"package name with a Greek upsilon", "req" + grUpsilon + "ests",
			[]finding{{"req" + grUpsilon + "ests", 0, []string{"Greek", "Latin"}, []rune{0x03C5}}}},
		{"package name with Cyrillic letters, in a line", "bump p" + cyrA + "yp" + cyrA + "l from 1.0 to 1.1",
			[]finding{{"p" + cyrA + "yp" + cyrA + "l", 5, []string{"Cyrillic", "Latin"}, []rune{0x0430, 0x0430}}}},
		{"host in a URL", "https://g" + cyrO + cyrO + "gle.com/watch?v=abc",
			[]finding{{"g" + cyrO + cyrO + "gle", 8, []string{"Cyrillic", "Latin"}, []rune{0x043E, 0x043E}}}},
		{"module path: only the mixed segment", "github.com/" + cyrA + "pple/swift",
			[]finding{{cyrA + "pple", 11, []string{"Cyrillic", "Latin"}, []rune{0x0430}}}},
		{"hyphen and underscore split words", "left-p" + cyrA + "d_right",
			[]finding{{"p" + cyrA + "d", 5, []string{"Cyrillic", "Latin"}, []rune{0x0430}}}},
		{"three scripts", "a" + cyrA + grOmicron,
			[]finding{{"a" + cyrA + grOmicron, 0, []string{"Cyrillic", "Greek", "Latin"}, []rune{0x0430, 0x03BF}}}},
		{"Armenian in a Latin word", "g" + armO + "lang",
			[]finding{{"g" + armO + "lang", 0, []string{"Armenian", "Latin"}, []rune{0x0585}}}},
		{"two findings", grUpsilon + "rl and p" + cyrA + "th",
			[]finding{
				{grUpsilon + "rl", 0, []string{"Greek", "Latin"}, []rune{0x03C5}},
				{"p" + cyrA + "th", 9, []string{"Cyrillic", "Latin"}, []rune{0x0430}},
			}},
		{"digits do not count", "v2" + cyrA + "3b",
			[]finding{{"v2" + cyrA + "3b", 0, []string{"Cyrillic", "Latin"}, []rune{0x0430}}}},

		// What it reports although it is honest, and what it cannot see.
		{"honest prose is reported too: a Greek mu before a Latin unit", "5" + grMu + "m",
			[]finding{{"5" + grMu + "m", 0, []string{"Greek", "Latin"}, []rune{0x03BC}}}},
		{"not seen: a word wholly in Cyrillic that reads as Latin", cyrA + cyrP + cyrP + cyrPalochk + cyrE, nil},
		{"not seen: fullwidth Latin letters", string(rune(0xFF52)) + string(rune(0xFF45)) + "quests", nil},
		{"not seen: mathematical letters, which have no script", string(rune(0x1D42B)) + "equests", nil},
		{"not seen: look-alikes within Latin", "rnicrosoft Iinux", nil},

		// Scripts that are written together.
		{"Japanese: Han, Hiragana, Katakana and Latin in one word",
			"T" + string(rune(0x30B7)) + string(rune(0x30E3)) + string(rune(0x30C4)) + string(rune(0x306E)) + string(rune(0x5E97)), nil},
		{"Korean: Hangul with Han and Latin", "K" + string(rune(0xD55C)) + string(rune(0x570B)), nil},
		{"Han with Cyrillic is not written together", string(rune(0x5E97)) + cyrA,
			[]finding{{string(rune(0x5E97)) + cyrA, 0, []string{"Cyrillic", "Han"}, []rune{0x5E97}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := Mark.Line(tc.text)
			if text.String() != tc.text {
				t.Fatalf("cleaning changed the text: %+q", text.String())
			}
			var got []finding
			for _, m := range text.MixedScripts() {
				got = append(got, finding{m.Word.String(), m.Start, m.Scripts, m.Odd})
				if end := m.Start + len(m.Word.String()); text.String()[m.Start:end] != m.Word.String() {
					t.Errorf("Start %d does not point at the word %+q", m.Start, m.Word.String())
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("MixedScripts(%+q)\n got %+v\nwant %+v", tc.text, got, tc.want)
			}
			// It reports; it never alters.
			if text.String() != tc.text {
				t.Errorf("the text changed: %+q", text.String())
			}
		})
	}
}

func TestMixed_String(t *testing.T) {
	text := Drop.Line("pip install req" + grUpsilon + "ests p" + cyrA + "yp" + cyrA + "l")
	mixed := text.MixedScripts()
	if len(mixed) != 2 {
		t.Fatalf("got %d findings, want 2", len(mixed))
	}
	want := []string{
		`"req` + grUpsilon + `ests" mixes Greek and Latin letters (U+03C5 is Greek)`,
		`"p` + cyrA + `yp` + cyrA + `l" mixes Cyrillic and Latin letters (U+0430 is Cyrillic, U+0430 is Cyrillic)`,
	}
	for i, m := range mixed {
		if got := m.String(); got != want[i] {
			t.Errorf("String() = %+q, want %+q", got, want[i])
		}
	}
}

// TestMixedScripts_AfterMarking: a marker stands where a control was, and its
// letters are ASCII. It must not make the word around it look mixed, nor hide
// that it is.
func TestMixedScripts_AfterMarking(t *testing.T) {
	plain := Mark.Line(cyrP + cyrA + rlo + cyrP + cyrA)
	if got := plain.String(); got != cyrP+cyrA+"<U+202E>"+cyrP+cyrA {
		t.Fatalf("marked text = %+q", got)
	}
	if mixed := plain.MixedScripts(); len(mixed) != 0 {
		t.Errorf("a marker inside a Cyrillic word made it look mixed: %v", mixed)
	}

	mixed := Mark.Line("p" + cyrA + zwsp + "y").MixedScripts()
	if len(mixed) != 1 || mixed[0].Word.String() != "p"+cyrA {
		t.Errorf("findings = %v, want the mixed half before the marker", mixed)
	}
}
