package termsafe

import (
	"math/rand"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

const (
	esc = "\x1b"
	st  = esc + `\`
	bel = "\x07"
)

// cleanCases is the table both Clean and Line are held to. want is what Clean
// returns; none of these inputs holds a newline or tab unless the case is
// about one, so Line returns the same text with those turned into spaces.
var cleanCases = []struct {
	name string
	in   string
	want string
}{
	// Text that must come through untouched.
	{"empty", "", ""},
	{"ascii", "[download] 42.0% of 10.00MiB at 1.00MiB/s ETA 00:05", "[download] 42.0% of 10.00MiB at 1.00MiB/s ETA 00:05"},
	{"utf8", "naïve café — 日本語 🎬 Ünïcödé", "naïve café — 日本語 🎬 Ünïcödé"},
	{"combining mark", "e\u0301", "e\u0301"},
	{"zwj emoji", "👩\u200d💻", "👩\u200d💻"},
	{"replacement char in the input", "a\ufffdb", "a\ufffdb"},
	{"nbsp and latin-1 supplement start", "a\u00a0b", "a\u00a0b"},
	{"newline and tab", "a\tb\nc", "a\tb\nc"},

	// Carriage returns.
	{"crlf", "a\r\nb", "a\nb"},
	{"lone cr", "10%\r20%", "10%\n20%"},
	{"trailing cr", "abc\r", "abc\n"},

	// CSI.
	{"csi erase display", "a" + esc + "[2Jb", "ab"},
	{"csi cursor position", "a" + esc + "[10;20Hb", "ab"},
	{"csi cursor up", "a" + esc + "[5Ab", "ab"},
	{"csi erase line", "a" + esc + "[2Kb", "ab"},
	{"csi sgr", esc + "[1;31mred" + esc + "[0m", "red"},
	{"csi private mode", "a" + esc + "[?1049hb", "ab"},
	{"csi colon params", "a" + esc + "[38:2::255:0:0mb", "ab"},
	{"csi with intermediate", "a" + esc + "[ qb", "ab"},
	{"csi device status report", "a" + esc + "[6nb", "ab"},
	{"csi unterminated at end", "abc" + esc + "[12;", "abc"},
	{"csi bare at end", "abc" + esc + "[", "abc"},
	{"csi cut by newline", "a" + esc + "[12\nb", "a\nb"},
	{"csi cut by another escape", "a" + esc + "[1" + esc + "[2Jb", "ab"},
	{"csi cut by non-ascii", "a" + esc + "[1é", "aé"},

	// OSC.
	{"osc 52 clipboard write, bel", "a" + esc + "]52;c;aGVsbG8=" + bel + "b", "ab"},
	{"osc 52 clipboard write, st", "a" + esc + "]52;c;aGVsbG8=" + st + "b", "ab"},
	{"osc 0 window title", esc + "]0;pwned" + bel + "title", "title"},
	{"osc 8 hyperlink keeps its text", esc + "]8;;https://evil.example/" + st + "click here" + esc + "]8;;" + st + " end", "click here end"},
	{"osc 8 hyperlink with id, bel", esc + "]8;id=1;https://evil.example/?a=1;b=2" + bel + "link" + esc + "]8;;" + bel, "link"},
	{"osc 9 notification", "a" + esc + "]9;you have been pwned" + bel + "b", "ab"},
	{"osc unterminated at end", "a" + esc + "]52;c;aGVsbG8=", "a"},
	{"osc unterminated ends at newline", "a" + esc + "]0;x\nb", "a\nb"},
	{"osc ended by c1 st", "a" + esc + "]0;t\u009cb", "ab"},

	// DCS, APC, PM, SOS: the strings bubbletea 2.1 replays (F1).
	{"dcs terminated", "a" + esc + "P+q544e" + st + "b", "ab"},
	{"dcs unterminated", "a" + esc + "P+q544e", "a"},
	{"dcs unterminated ends at newline", "a" + esc + "P+q544e\nnext", "a\nnext"},
	{"dcs unterminated ends at cr", "a" + esc + "P+q544e\rnext", "a\nnext"},
	{"dcs sixel", "a" + esc + "Pq#0;2;0;0;0#0~~@@vv@@~~$" + st + "b", "ab"},
	{"dcs is not ended by bel", "a" + esc + "Pq" + bel + "still payload" + st + "b", "ab"},
	{"dcs cut by a csi", "a" + esc + "Pq" + esc + "[2Jb", "ab"},
	{"dcs holding an osc", "a" + esc + "P" + esc + "]0;t" + bel + "b", "ab"},
	{"apc terminated (kitty graphics)", "a" + esc + "_Gf=100;AAAA" + st + "b", "ab"},
	{"apc unterminated", "a" + esc + "_Gf=100;AAAA", "a"},
	{"pm terminated", "a" + esc + "^secret" + st + "b", "ab"},
	{"pm unterminated", "a" + esc + "^secret", "a"},
	{"sos terminated", "a" + esc + "Xsecret" + st + "b", "ab"},
	{"sos unterminated", "a" + esc + "Xsecret", "a"},

	// Other escapes.
	{"ris full reset", "a" + esc + "cb", "ab"},
	{"charset designation", "a" + esc + "(Bb", "ab"},
	{"dec save and restore cursor", "a" + esc + "7b" + esc + "8c", "abc"},
	{"reverse index", "a" + esc + "Mb", "ab"},
	{"dec alignment test", "a" + esc + "#8b", "ab"},
	{"lone esc at end", "abc" + esc, "abc"},
	{"esc before newline", "a" + esc + "\nb", "a\nb"},
	{"esc esc csi", "a" + esc + esc + "[2Jb", "ab"},
	{"stray st", "a" + st + "b", "ab"},
	{"esc before non-ascii", "a" + esc + "é", "aé"},

	// C0 controls and DEL.
	{"bel bs nul vt ff del", "a\x07b\x08c\x00d\x0be\x0cf\x7fg", "abcdefg"},
	{"shift out and in", "a\x0eb\x0fc", "abc"},
	{"can and sub", "a\x18b\x1ac", "abc"},
	{"every c0 but tab and newline", "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1c\x1d\x1e\x1fok", "ok"},

	// C1 controls as code points (UTF-8 C2 80 .. C2 9F): a terminal that
	// honours them reads these exactly like their ESC forms.
	{"c1 csi", "a\u009b2Jb", "ab"},
	{"c1 csi sgr", "\u009b31mred\u009b0m", "red"},
	{"c1 osc 52 ended by c1 st", "a\u009d52;c;aGVsbG8=\u009cb", "ab"},
	{"c1 osc ended by bel", "a\u009d0;t" + bel + "b", "ab"},
	{"c1 dcs ended by c1 st", "a\u0090+q544e\u009cb", "ab"},
	{"c1 dcs ended by esc st", "a\u0090+q544e" + st + "b", "ab"},
	{"c1 dcs unterminated", "a\u0090payload", "a"},
	{"c1 apc", "a\u009fGf=100;AAAA\u009cb", "ab"},
	{"c1 pm", "a\u009esecret\u009cb", "ab"},
	{"c1 sos", "a\u0098secret\u009cb", "ab"},
	{"c1 stray st", "a\u009cb", "ab"},
	{"c1 nel and pad", "a\u0085b\u0080c", "abc"},
	{"every c1", "\u0080\u0081\u0082\u0083\u0084\u0085\u0086\u0087\u0088\u0089\u008a\u008b\u008c\u008d\u008e\u008f\u0091\u0092\u0093\u0094\u0095\u0096\u0097\u0099\u009a\u009cok", "ok"},

	// C1 controls as raw 8-bit bytes. They are not valid UTF-8, so each
	// becomes U+FFFD and what followed it stays as inert text: the same bytes
	// are ordinary letters in the legacy code pages yt-dlp writes on Windows.
	{"raw 8-bit csi", "a\x9b2Jb", "a\ufffd2Jb"},
	{"raw 8-bit dcs and st", "a\x90q\x9cb", "a\ufffdq\ufffdb"},
	{"raw 8-bit osc 52", "a\x9d52;c;QUJD" + bel + "b", "a\ufffd52;c;QUJDb"},
	{"raw 8-bit apc", "a\x9fG\x9cb", "a\ufffdG\ufffdb"},

	// Invalid UTF-8.
	{"truncated utf8 at end", "caf\xc3", "caf\ufffd"},
	{"latin-1 byte", "na\xefve", "na\ufffdve"},
	{"overlong esc", "a\xc0\x9bb", "a\ufffd\ufffdb"},
	{"overlong encoded c1", "a\xe0\x82\x9b2Jb", "a\ufffd\ufffd\ufffd2Jb"},
	{"utf8 surrogate", "a\xed\xa0\x80b", "a\ufffd\ufffd\ufffdb"},

	// Mixed with UTF-8.
	{"utf8 around sequences", "日本" + esc + "[31m語" + esc + "[0m 🎬" + esc + "P+q" + st + " café", "日本語 🎬 café"},
	{"sequence before a combining mark", "e" + esc + "[1m\u0301", "e\u0301"},
	{"utf8 inside a dropped payload", "a" + esc + "]0;タイトル" + bel + "b", "ab"},
	{"utf8 right after an introducer", "a" + esc + "P日本" + st + "語", "a語"},
	{"hostile yt-dlp line", "[download] " + esc + "P+q544e" + st + "DCS " + esc + "_Gf=100;AAAA" + st + "APC " + esc + "]52;c;QUJD" + bel + "OSC " + esc + "[2JCSI", "[download] DCS APC OSC CSI"},
}

func TestClean(t *testing.T) {
	for _, tc := range cleanCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Drop.Clean(tc.in).String(); got != tc.want {
				t.Errorf("Drop.Clean(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestLine holds Line to the same table: one line out, so the newlines and
// tabs Clean keeps become spaces.
func TestLine(t *testing.T) {
	oneLine := strings.NewReplacer("\n", " ", "\t", " ")
	for _, tc := range cleanCases {
		t.Run(tc.name, func(t *testing.T) {
			want := oneLine.Replace(tc.want)
			if got := Drop.Line(tc.in).String(); got != want {
				t.Errorf("Drop.Line(%q)\n got %q\nwant %q", tc.in, got, want)
			}
		})
	}
}

func TestLine_Whitespace(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"newline", "a\nb", "a b"},
		{"tab", "a\tb", "a b"},
		{"crlf is one space", "a\r\nb", "a b"},
		{"lone cr", "a\rb", "a b"},
		{"file name with a newline", "title\nwith newline [id].mp4", "title with newline [id].mp4"},
		{"runs are kept", "a\n\n\tb", "a   b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Drop.Line(tc.in).String(); got != tc.want {
				t.Errorf("Drop.Line(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSplitAcrossLines covers a sequence that starts in one log line and ends
// in the next. Each line is cleaned by itself, as a line-oriented reader
// delivers them. Nothing cleaned can complete a sequence when the pieces are
// put back together, and cleaning the joined text gives the same result.
func TestSplitAcrossLines(t *testing.T) {
	cases := []struct {
		name          string
		first, second string
		wantFirst     string
		wantSecond    string
	}{
		{"dcs", "Downloading " + esc + "P+q54", "4e" + st + " done", "Downloading ", "4e done"},
		{"apc", "a" + esc + "_Gf=100;AA", "AA" + st + "b", "a", "AAb"},
		{"osc 52", "x" + esc + "]52;c;aGVs", "bG8=" + bel + "y", "x", "bG8=y"},
		{"osc 8", "see " + esc + "]8;;https://evil", ".example/" + st + "link", "see ", ".example/link"},
		{"csi", "a" + esc + "[", "2Jb", "a", "2Jb"},
		{"csi params", "a" + esc + "[38;5", ";196mred", "a", ";196mred"},
		{"esc alone", "a" + esc, "[2Jb", "a", "[2Jb"},
		{"esc then st", "a" + esc + "Pq" + esc, `\b`, "a", `\b`},
		{"c1 dcs", "a\u0090+q54", "4e\u009cb", "a", "4eb"},
		{"utf8 either side", "日本" + esc + "P語", "タイトル" + st + "🎬", "日本", "タイトル🎬"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first, second := Drop.Clean(tc.first).String(), Drop.Clean(tc.second).String()
			if first != tc.wantFirst {
				t.Errorf("Drop.Clean(first %q) = %q, want %q", tc.first, first, tc.wantFirst)
			}
			if second != tc.wantSecond {
				t.Errorf("Drop.Clean(second %q) = %q, want %q", tc.second, second, tc.wantSecond)
			}
			for _, joined := range []string{first + second, first + "\n" + second} {
				if r, bad := firstUnsafe(joined, false); bad {
					t.Errorf("rejoined %q holds %U", joined, r)
				}
			}
			whole := Drop.Clean(tc.first + "\n" + tc.second).String()
			if want := tc.wantFirst + "\n" + tc.wantSecond; whole != want {
				t.Errorf("Drop.Clean(first + newline + second) = %q, want %q", whole, want)
			}
		})
	}
}

func TestText_ZeroValue(t *testing.T) {
	var z Text
	if z.String() != "" || !z.IsEmpty() {
		t.Errorf("zero Text = %q, IsEmpty %v; want empty", z.String(), z.IsEmpty())
	}
	if Drop.Clean("x").IsEmpty() {
		t.Error(`Drop.Clean("x").IsEmpty() = true`)
	}
	if !Drop.Clean(esc + "[2J").IsEmpty() {
		t.Error("a text that was only a sequence is not empty")
	}
}

func TestText_TrimSpace(t *testing.T) {
	if got := Drop.Clean("  \tline \n").TrimSpace().String(); got != "line" {
		t.Errorf("TrimSpace() = %q, want %q", got, "line")
	}
}

func TestText_Truncate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"shorter", "abc", 5, "abc"},
		{"exact", "abcde", 5, "abcde"},
		{"cut", "abcdefgh", 5, "ab..."},
		{"cut counts runes, not bytes", "日本語日本語", 5, "日本..."},
		{"no room for the dots", "abcdef", 3, "abc"},
		{"one", "日本語", 1, "日"},
		{"zero", "abc", 0, ""},
		{"negative", "abc", -1, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Drop.Line(tc.in).Truncate(tc.max).String()
			if got != tc.want {
				t.Errorf("Truncate(%d) of %q = %q, want %q", tc.max, tc.in, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("Truncate(%d) of %q = %q, not valid UTF-8", tc.max, tc.in, got)
			}
		})
	}
}

// firstUnsafe returns the first rune of s that must never reach a terminal:
// a C0 control (newline and tab excepted unless oneLine), DEL, a C1 control,
// a byte that is not valid UTF-8, a line or paragraph separator, or a
// variation selector that does not stand directly after a character it can
// modify.
func firstUnsafe(s string, oneLine bool) (rune, bool) {
	var before rune
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			return rune(s[i]), true
		case r == '\n' || r == '\t':
			if oneLine {
				return r, true
			}
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			return r, true
		case r == 0x2028 || r == 0x2029:
			return r, true
		case unicode.Is(unicode.Variation_Selector, r):
			if unicode.Is(unicode.Variation_Selector, before) || !takesSelector(before, r) {
				return r, true
			}
		}
		before = r
		i += size
	}
	return 0, false
}

// firstNotASCII returns the first byte of s that is not printable ASCII.
// Newline and tab pass unless oneLine.
func firstNotASCII(s string, oneLine bool) (byte, bool) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c == '\n' || c == '\t') && !oneLine {
			continue
		}
		if c < 0x20 || c > 0x7e {
			return c, true
		}
	}
	return 0, false
}

// checkInvariants is what every input must satisfy, hostile or not.
func checkInvariants(t *testing.T, in string) {
	t.Helper()
	clean, line := Drop.Clean(in).String(), Drop.Line(in).String()
	if r, bad := firstUnsafe(clean, false); bad {
		t.Fatalf("Drop.Clean(%q) = %q holds %U", in, clean, r)
	}
	if r, bad := firstUnsafe(line, true); bad {
		t.Fatalf("Drop.Line(%q) = %q holds %U", in, line, r)
	}
	if again := Drop.Clean(clean).String(); again != clean {
		t.Fatalf("Clean is not idempotent on %q: %q then %q", in, clean, again)
	}
	// Whatever a policy returns, there is nothing left in it to remove
	// or mark, under either policy.
	for name, policy := range map[string]Policy{"Drop": Drop, "Mark": Mark} {
		out := policy.Clean(in).String()
		if r, bad := firstUnsafe(out, false); bad {
			t.Fatalf("%s.Clean(%q) = %q holds %U", name, in, out, r)
		}
		for _, again := range []Policy{Drop, Mark} {
			if text, rep := again.CleanReport(out); rep.Changed() || text.String() != out {
				t.Fatalf("%s.Clean(%+q) = %+q still has something to clean: %+v", name, in, out, rep)
			}
		}
	}
	if again := Drop.Line(line).String(); again != line {
		t.Fatalf("Line is not idempotent on %q: %q then %q", in, line, again)
	}
	if want := strings.NewReplacer("\n", " ", "\t", " ").Replace(clean); line != want {
		t.Fatalf("Drop.Line(%q) = %q, want Clean with spaces %q", in, line, want)
	}
	if got := Drop.Clean(in).Truncate(7).String(); !utf8.ValidString(got) {
		t.Fatalf("Truncate of Drop.Clean(%q) = %q, not valid UTF-8", in, got)
	} else if r, bad := firstUnsafe(got, false); bad {
		t.Fatalf("Truncate of Drop.Clean(%q) = %q holds %U", in, got, r)
	}

	// The ASCII form of anything cleaned is printable ASCII and nothing
	// else, it is its own ASCII form, and there is nothing left in it to
	// clean.
	for name, policy := range map[string]Policy{"Drop": Drop, "Mark": Mark} {
		for _, oneLine := range []bool{false, true} {
			text := policy.Clean(in)
			if oneLine {
				text = policy.Line(in)
			}
			ascii := text.ASCII()
			if c, bad := firstNotASCII(ascii.String(), oneLine); bad {
				t.Fatalf("%s, one line %v: ASCII of %+q = %q holds the byte %#x", name, oneLine, in, ascii.String(), c)
			}
			if again := ascii.ASCII(); again != ascii {
				t.Fatalf("ASCII is not idempotent on %+q: %q then %q", in, ascii.String(), again.String())
			}
			for _, again := range []Policy{Drop, Mark} {
				if recleaned, rep := again.CleanReport(ascii.String()); rep.Changed() || recleaned != ascii {
					t.Fatalf("%s: the ASCII form %q of %+q still has something to clean: %+v", name, ascii.String(), in, rep)
				}
			}
			if (text.String() == ascii.String()) != isASCII(text.String()) {
				t.Fatalf("ASCII changed %+q into %q", text.String(), ascii.String())
			}
		}
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func TestInvariants_Table(t *testing.T) {
	for _, tc := range cleanCases {
		checkInvariants(t, tc.in)
	}
}

// fragments are the pieces hostile input is made of; the random test and the
// fuzz seeds glue them together in every order.
var fragments = []string{
	esc, "[", "]", "P", "_", "^", "X", `\`, bel, "\n", "\r", "\t", " ", ";", "?",
	"0", "2", "5", "J", "m", "q", "c", "(", "#", "a", "é", "日", "🎬", "\u0301",
	"\u0090", "\u0098", "\u009b", "\u009c", "\u009d", "\u009e", "\u009f", "\u0085",
	"\x90", "\x9b", "\x9c", "\x9d", "\xc2", "\xc3", "\xe2", "\xf0", "\xff",
	"\x00", "\x08", "\x18", "\x1a", "\x7f", "\ufffd",
	// bidirectional controls, invisible characters, and what joiners and
	// tag characters may legitimately follow
	rlo, rli, pdi, lrm, zwsp, zwj, zwnj, bom, shy, vs16,
	string(rune(0x1F3F4)), string(rune(0xE0067)), string(rune(0xE007F)),
	string(rune(0x1F469)), string(rune(0x0645)), string(rune(0x0915)), string(rune(0x094D)), string(rune(0x0430)),
	// variation selectors of every kind, characters they belong after, and
	// the line breaks that are not a newline
	vs1Str, vs2Str, vs4Str, vs8Str, vs15, vs17, vs256, fvs1,
	heart, hanKatsu, mongolianA, hieroglyph, intersect, ideoComma, "0", "#",
	lineSep, paraSep,
}

// TestInvariants_Random builds inputs from fragments with a fixed seed, so a
// failure reproduces.
func TestInvariants_Random(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		var a, b strings.Builder
		for n := rng.Intn(12); n > 0; n-- {
			a.WriteString(fragments[rng.Intn(len(fragments))])
		}
		for n := rng.Intn(12); n > 0; n-- {
			b.WriteString(fragments[rng.Intn(len(fragments))])
		}
		first, second := a.String(), b.String()
		checkInvariants(t, first)

		// Cleaning line by line and cleaning the whole text agree, so it does
		// not matter how a caller's reader happened to split its input. A
		// trailing CR is left out: with the newline it forms one CRLF.
		if strings.HasSuffix(first, "\r") {
			continue
		}
		whole := Drop.Clean(first + "\n" + second).String()
		parts := Drop.Clean(first).String() + "\n" + Drop.Clean(second).String()
		if whole != parts {
			t.Fatalf("Drop.Clean(%q + newline + %q) = %q, line by line %q", first, second, whole, parts)
		}
	}
}

func FuzzClean(f *testing.F) {
	for _, tc := range cleanCases {
		f.Add(tc.in)
	}
	for _, tc := range formatCases {
		f.Add(tc.in)
	}
	for _, tc := range strayCases {
		f.Add(tc.in)
	}
	for _, s := range Samples() {
		f.Add(s.Text)
	}
	f.Fuzz(func(t *testing.T, in string) {
		checkInvariants(t, in)
	})
}

func TestReport(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Report
	}{
		{"clean ascii", "plain text", Report{}},
		{"clean utf8", "naïve 日本語 🎬", Report{}},
		{"newline, tab and carriage returns are not changes", "a\tb\r\nc\rd\n", Report{}},
		{"csi", "a" + esc + "[2Jb" + esc + "[0m", Report{Sequences: 2}},
		{"csi cut short", "a" + esc + "[12", Report{Sequences: 1}},
		{"osc 52", "a" + esc + "]52;c;aGVsbG8=" + bel, Report{Sequences: 1}},
		{"osc 8 hyperlink", esc + "]8;;https://evil.example/" + st + "link" + esc + "]8;;" + st, Report{Sequences: 2}},
		{"dcs terminated", "a" + esc + "P+q544e" + st + "b", Report{Sequences: 1}},
		{"dcs unterminated", "a" + esc + "P+q544e", Report{Unterminated: 1}},
		{"apc unterminated then a line with a csi", "a" + esc + "_G\nb" + esc + "[0m", Report{Unterminated: 1, Sequences: 1}},
		{"dcs cut by a csi", "a" + esc + "Pq" + esc + "[2Jb", Report{Unterminated: 1, Sequences: 1}},
		{"two unterminated on two lines", esc + "]0;x\n" + esc + "^y", Report{Unterminated: 2}},
		{"other escapes", "a" + esc + "c" + esc + "(B" + st, Report{Sequences: 3}},
		{"lone esc", "a" + esc, Report{Controls: 1}},
		{"esc before non-ascii", "a" + esc + "é", Report{Controls: 1}},
		{"c0 and del", "a\x07b\x08c\x00\x7f", Report{Controls: 4}},
		{"c1 csi", "a\u009b2Jb", Report{Sequences: 1}},
		{"c1 dcs unterminated", "a\u0090payload", Report{Unterminated: 1}},
		{"c1 single", "a\u0085b\u009c", Report{Controls: 2}},
		{"raw 8-bit bytes", "a\x9b2J\x90", Report{Invalid: 2}},
		{"truncated utf8", "caf\xc3", Report{Invalid: 1}},
		{"everything", esc + "[1m" + esc + "Pq" + "\n\x07\xff", Report{Sequences: 1, Unterminated: 1, Controls: 1, Invalid: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, got := Drop.CleanReport(tc.in)
			if got != tc.want {
				t.Errorf("Drop.CleanReport(%q) reported %+v, want %+v", tc.in, got, tc.want)
			}
			if text != Drop.Clean(tc.in) {
				t.Errorf("Drop.CleanReport(%q) text = %q, Clean gives %q", tc.in, text, Drop.Clean(tc.in))
			}
			line, got := Drop.LineReport(tc.in)
			if got != tc.want {
				t.Errorf("Drop.LineReport(%q) reported %+v, want %+v", tc.in, got, tc.want)
			}
			if line != Drop.Line(tc.in) {
				t.Errorf("Drop.LineReport(%q) text = %q, Line gives %q", tc.in, line, Drop.Line(tc.in))
			}
			if got.Changed() != (tc.want != Report{}) {
				t.Errorf("Changed() = %v for %+v", got.Changed(), got)
			}
		})
	}
}

func TestReport_String(t *testing.T) {
	cases := []struct {
		rep  Report
		want string
	}{
		{Report{}, ""},
		{Report{Sequences: 1}, "1 control sequence"},
		{Report{Sequences: 2}, "2 control sequences"},
		{Report{Unterminated: 1}, "1 unterminated control string"},
		{Report{Controls: 3, Invalid: 1}, "3 control characters and 1 invalid byte"},
		{Report{Sequences: 2, Unterminated: 2, Controls: 1, Invalid: 4},
			"2 control sequences, 2 unterminated control strings, 1 control character and 4 invalid bytes"},
	}
	for _, tc := range cases {
		if got := tc.rep.String(); got != tc.want {
			t.Errorf("%+v.String() = %q, want %q", tc.rep, got, tc.want)
		}
		if r, bad := firstUnsafe(tc.rep.String(), true); bad {
			t.Errorf("%+v.String() holds %U", tc.rep, r)
		}
	}
}

// TestReport_Invariants ties the report to the text: nothing reported means
// nothing but white space changed, and something reported means the text did
// change.
func TestReport_Invariants(t *testing.T) {
	whitespace := strings.NewReplacer("\r\n", "\n", "\r", "\n", lineSep, "\n", paraSep, "\n")
	rng := rand.New(rand.NewSource(2))
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		for n := rng.Intn(12); n > 0; n-- {
			b.WriteString(fragments[rng.Intn(len(fragments))])
		}
		in := b.String()
		text, rep := Drop.CleanReport(in)
		same := text.String() == whitespace.Replace(in)
		if rep.Changed() == same {
			t.Fatalf("Drop.CleanReport(%q) = %q with report %+v", in, text, rep)
		}
	}
}

// TestCleanInputAllocatesNothing guards the fast path: text with nothing to
// remove is returned as it is, with or without a report.
func TestCleanInputAllocatesNothing(t *testing.T) {
	const in = "[download]  42.0% of 10.00MiB at 1.00MiB/s ETA 00:05"
	var sink Text
	var rep Report
	for name, fn := range map[string]func(){
		"Drop.Clean":       func() { sink = Drop.Clean(in) },
		"Drop.Line":        func() { sink = Drop.Line(in) },
		"Drop.CleanReport": func() { sink, rep = Drop.CleanReport(in) },
		"Mark.LineReport":  func() { sink, rep = Mark.LineReport(in) },
		"Drop.Line, ASCII": func() { sink = Drop.Line(in).ASCII() },
		"Terminal.Fit":     func() { sink = Terminal{}.Fit(Mark.Clean(in)) },
	} {
		if n := testing.AllocsPerRun(100, fn); n != 0 {
			t.Errorf("%s allocates %v times on clean input", name, n)
		}
	}
	_, _ = sink, rep
}
