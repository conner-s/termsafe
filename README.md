# termsafe

termsafe makes text a program did not write safe to draw in a terminal: a
subprocess's output, a title from a remote site, a file name, an error that
quotes any of those.

A terminal acts on control sequences wherever they turn up, and a TUI
framework may pass them through. bubbletea 2.1 replays terminated DCS, APC,
PM and SOS strings it finds in a view's text. Text can also mislead without
controlling anything: a bidirectional override reorders what the reader sees,
and an invisible character makes two different names look the same. termsafe
removes the first kind and removes or marks the second.

It is one Go package with no dependencies outside the standard library.

```go
import "forgejo.kube.internal.conn3r.xyz/connerlab/termsafe"
```

`go doc` on the package lists every sequence and character it handles.
[PORTING.md](PORTING.md) says how to put it into a program.
[CHANGELOG.md](CHANGELOG.md) lists the releases.

## What it is not

- It is not an HTML sanitiser, a shell quoter or a file name cleaner.
  `<script>` and `$(rm -rf ~)` are printable text and come through
  untouched. A `Text` is safe on a terminal's screen and nowhere else.
- It does not stand between a child process and a terminal the child was
  given. It cleans strings the program holds.
- It does not detect names made of look-alike letters, beyond the one check
  described under [MixedScripts](#mixedscripts-and-what-it-misses).
- It does not measure display width, wrap, or normalise Unicode.

## Example

```go
// A file name a remote site chose: a right-to-left override that turns
// "fdp.exe" around for the reader, and an OSC 52 sequence that would write
// to the clipboard.
const hostileName = "invoice" + string(rune(0x202E)) + "fdp.exe\x1b]52;c;aGk=\x07"

// A view takes a Text, so the compiler refuses a raw string:
// drawTitle("raw") does not build.
func drawTitle(title termsafe.Text) {
	fmt.Println(title)
}

func main() {
	drawTitle(termsafe.Mark.Line(hostileName)) // invoice<U+202E>fdp.exe
	drawTitle(termsafe.Drop.Line(hostileName)) // invoicefdp.exe

	_, report := termsafe.Drop.LineReport(hostileName)
	if report.Changed() {
		// removed: 1 control sequence and 1 bidirectional control
		fmt.Println("removed:", report)
	}
}
```

`example_test.go` has the same calls as examples that run with the tests.

## The API

```go
type Policy int
const (
	Mark Policy = iota // the zero value
	Drop
)
func (p Policy) Clean(s string) Text                 // may span lines
func (p Policy) Line(s string) Text                  // one line
func (p Policy) CleanReport(s string) (Text, Report)
func (p Policy) LineReport(s string) (Text, Report)

type Text struct{ /* no exported fields */ }
func (t Text) String() string
func (t Text) IsEmpty() bool
func (t Text) TrimSpace() Text
func (t Text) Truncate(max int) Text
func (t Text) MixedScripts() []Mixed

type Report struct{ Sequences, Unterminated, Controls, Invalid, Bidi, Invisible int }
func (r Report) Changed() bool
func (r Report) String() string

type Mixed struct {
	Word    Text
	Start   int
	Scripts []string
	Odd     []rune
}
func (m Mixed) String() string

// The test kit, for a program's own tests.
type Sample struct{ Name, Text string }
const SampleMarker = "VISIBLE"
func Samples() []Sample
func ViewProblem(view string) string
func TerminalProblem(written string) string
```

## Guarantees

The right column names what holds each one: the type, or tests in this
repository.

| Guarantee | Held by |
| --- | --- |
| A `Text` with anything in it comes only from cleaning a string under a `Policy`, or from a method of another `Text`. A function that takes a `Text` cannot be handed a raw string. | The type: its one field is unexported. |
| A `Text` holds no ESC, no other C0 control, no DEL and no C1 control. `Clean` keeps newline and tab; `Line` turns them into spaces. With no ESC and no C1 in it, it holds no control sequence, and joining two cleaned texts cannot form one. | `TestClean`, `TestLine`, `TestInvariants_Random`, `TestSplitAcrossLines`, `FuzzClean` |
| A `Text` holds only valid UTF-8. Each invalid byte becomes U+FFFD. | the same |
| A `Text` holds no bidirectional control and no invisible format character, under either policy. `Drop` removes them, `Mark` replaces each with a marker such as `<U+202E>`. Three kinds stay because legitimate text needs them: a zero width joiner or non-joiner after a character it can act on, tag characters inside an emoji flag, and variation selectors. | `TestFormatCharacters`, `TestNoFormatCharacterSurvives` (every code point), `TestBidiControlsAreTheTwelve` |
| Cleaning a cleaned text changes nothing and reports nothing, under either policy. | `checkInvariants`, run by the tables, the random tests and the fuzz target |
| A control string without a terminator ends at the end of its line. Cleaning `a + "\n" + b` gives the cleaned `a`, a newline and the cleaned `b`, so it does not matter how a reader split its input into lines. The one exception is an `a` that ends in a carriage return, which forms one CR LF with the newline. | `TestInvariants_Random`, `TestSplitAcrossLines` |
| Printable text comes through byte for byte, including right-to-left text, emoji joined by zero width joiners, flags, and the joiners Persian and Indic scripts need. | `TestLegitimateTextIsUntouched` |
| A `Report` holds six counts and nothing else. No byte that was removed can leave through it. `Report.String()` is ASCII. | The type; `TestReport_String` |
| Input that needs no cleaning (printable ASCII, and for `Clean` newline and tab) is returned as the same string, with no allocation. | `TestCleanInputAllocatesNothing` |
| `MixedScripts` never changes the text. | `TestMixedScripts` |

Two of these have edges worth knowing.

"Invisible format character" is defined by the Unicode tables of the Go
release the program is built with. Go 1.26 has Unicode 15.0 and Go 1.27
has Unicode 17.0, so the same input can clean differently in two builds. A
character a later Unicode version adds is not known until Go knows it.

`Truncate` counts code points. It never cuts inside one, but it can cut an
emoji sequence or part a combining mark from its letter, and it knows nothing
about how many columns a character takes.

## The two policies

A policy answers one question: what happens to bidirectional controls and
invisible characters. Control sequences, control characters and invalid
bytes are removed the same way under both. There is no default and no way to
clean without naming a policy.

Use `Mark` for text a person bases a decision on: a title, a package name, a
version, a diff, a model's verdict. Each character is replaced by a marker
that names it, `<U+202E>`. The text then reads in the order it is stored and
the reader sees that something was there, and where. `Mark` is the zero
value, so a `Policy` nobody set hides nothing.

Use `Drop` where the text is only watched going by, such as a tool's log or
a progress line, and where the cleaned result is then used and must not hold
a marker, such as a URL the user typed. Nothing on the screen shows that the
text was touched. If the user should know, use the report.

A marker is ordinary ASCII. Input that already spells `<U+202E>` in letters
looks the same on screen as a marked override. The report tells them apart:
its `Bidi` count is zero for the first.

## The report

`CleanReport` and `LineReport` return the same `Text` as `Clean` and `Line`,
and a `Report` of what was taken out:

- `Sequences`: control sequences removed whole.
- `Unterminated`: OSC, DCS, APC, PM and SOS strings with no terminator. Each
  was cut at the end of its line or at the next escape, and the text in
  between went with it. This is the one case where text that may have been
  meant to be read is lost, so it is the one a program should mention.
- `Controls`: single control characters dropped.
- `Invalid`: bytes that were not valid UTF-8, each replaced by U+FFFD.
- `Bidi`, `Invisible`: counted whether the policy dropped or marked them.

The zero `Report` means the text came through as it was, apart from white
space: a carriage return made a newline, or `Line` made spaces of newlines
and tabs. Those are not counted. `String()` puts the counts in words, "2
control sequences and 1 bidirectional control".

## MixedScripts and what it misses

`Text.MixedScripts()` returns the words whose letters come from scripts that
do not belong together, such as a Latin word with one Greek or Cyrillic
letter in it. It is for names: a package, a host, a repository, a version.
It reports and never alters.

A word is a run of letters, combining marks and digits. Only letters count
towards its scripts. Latin with Han, Hiragana and Katakana, with Han and
Hangul, or with Han and Bopomofo are written together and are not reported.

This is the check the standard library allows. It knows the script of every
character but has no table of which characters look alike. So the check
misses:

- A word wholly in another script. `apple` spelled in five Cyrillic letters
  has one script and is not reported.
- A name whose parts are each in one script. Every word is judged alone, so
  a host with a Latin label, a dot and an all-Cyrillic label passes.
- Look-alikes inside one script: `l` and `I`, `rn` and `m`.
- Compatibility forms that Unicode files under Latin or under no script:
  fullwidth letters, mathematical letters.
- Digits. A digit of another script inside a Latin word does not count, and
  neither does `0` for `O`.

And it reports some honest text: a Greek mu, U+03BC, written before a Latin
unit of length.

Closing the gap takes Unicode's confusables data (UTS #39). It maps each
character to a prototype, so that two strings that look alike reduce to the
same "skeleton" and compare equal. Three things would have to be added:

1. The table, a few thousand entries, generated into this package from
   `confusables.txt` and regenerated with each Unicode version. Generated
   data keeps the package free of dependencies.
2. A check that a word in a single non-Latin script has the skeleton of a
   Latin word. That finds the whole-script case.
3. For look-alikes inside one script, a list of the names to protect. `rn`
   reads as `m` only if a name with an `m` exists that someone would want to
   imitate, and only the calling program knows its names.

Where a name must be ASCII by the rules of its registry, test for that. It
is stricter and simpler than any of this.

## What comes through

termsafe leaves all of this alone. Whether that matters depends on the
program.

- Variation selectors, U+FE00 to U+FE0F and U+E0100 to U+E01EF, anywhere and
  in any number. Emoji and ideographs need them. A run of them after any
  character draws nothing and can carry hidden data, the way the tag
  characters can, which termsafe does remove.
- Characters that draw as blank space: the no-break space, the spaces of
  other widths U+2000 to U+200A, the ideographic space U+3000, the braille
  blank U+2800.
- The line and paragraph separators U+2028 and U+2029, under `Line` too. A
  program that reads the same text later may break a line at them.
- Private-use and unassigned code points. What a terminal draws for them
  depends on its font.

One assumption sits under everything: the terminal decodes UTF-8. Ordinary
characters hold the bytes 0x80 to 0x9F in their encodings (U+00DB is C3 9B),
and a terminal in an 8-bit mode reads those bytes as C1 controls. No
cleaning short of ASCII-only output helps there.

## What was tested, and what was not

Tested in this repository, by CI on every push to `main` and every pull
request:

- Tables of hostile input with the exact output expected: CSI, OSC, DCS,
  APC, PM and SOS in their ESC and C1 forms, terminated and not, cut by a
  newline or by another escape; every C0 and C1 control; raw 8-bit bytes
  and broken UTF-8; the Trojan Source shapes; invisible characters; both
  policies; the report for each.
- Every code point from U+0080 up, placed between two ASCII letters: none
  that is a bidirectional control or an invisible character survives.
- Two runs of 20,000 inputs glued from hostile fragments, with fixed seeds.
  One checks the guarantees above. The other checks that the report says
  something exactly when the text changed.
- A fuzz target over the same checks, seeded with the control sequence
  table, for 30 seconds per run. Nobody has run a long fuzzing campaign.
- The test kit against the cleaner. Cleaned, all 24 samples pass both
  checks. Raw, every sample fails the view check and all but two fail the
  terminal check: a renderer writes CSI sequences of its own, and a joiner
  between Latin letters is judged in the view.
- Statement coverage of the package is 100% and CI fails below that.

Tested in the programs that use it, not here:

- glip feeds every sample through each of the seven ways outside text
  enters its TUI and checks the view's string. For six of them it also runs
  a real bubbletea program with a buffer in place of the terminal and checks
  every byte the program wrote. glip replays captured output of yt-dlp and
  ffmpeg through the same path as well. PORTING.md describes both tests so
  each program can write its own.

Not tested:

- No terminal emulator has been driven with cleaned text. The claim that a
  terminal cannot act on text that holds no C0 and no C1 control rests on
  how terminals parse (ECMA-48), not on an experiment with xterm, kitty or
  Windows Terminal.
- How a terminal that implements the bidirectional algorithm draws
  right-to-left text termsafe leaves alone.
- Speed, beyond the one test that clean ASCII allocates nothing. Cleaning is
  one pass over the string.
- Windows. CI runs the tests on Linux and they were run on macOS. For
  Windows there is only `go vet` with `GOOS=windows`. The package has no code
  specific to an operating system.
- Nobody outside the project has reviewed it. The owner reviewed it in
  connerlab/glip#12, where it was written.

## Stability

The module is at v0. What that means here:

- The guarantees above hold in every release. A release that broke one
  would be a bug, fixed in a patch release.
- The API can still change, and so can what cleaning does with text that
  was legitimate before. Either happens only in a minor release, v0.x.0, and
  CHANGELOG.md says so under "Changed". A patch release, v0.x.y, does
  neither.
- A fix for something hostile that got through can come in any release, and
  it makes cleaning remove more than before. A test in a program that pins
  the exact output for hostile input can then fail. Test with the kit
  instead: `ViewProblem` and `TerminalProblem` say whether anything got
  through, whatever the cleaner did with it.
- `Samples()` grows. Every behaviour change brings a new sample, and a
  program's two tests pick it up when they take the release.
- A build with a newer Go can remove characters an older one let through,
  because the Unicode tables come from the standard library.
- v1.0.0 comes when the programs that use it have stopped asking for API
  changes.

## Releases

A release is an annotated git tag `vMAJOR.MINOR.PATCH` on a commit of `main`
that CI passed, made after CHANGELOG.md has a section for that version. Go
fetches the tag from this repository, so there is nothing to publish.

A tag is never moved or deleted. Programs record a version's checksum in
their `go.sum`, and a moved tag breaks their builds. A bad release gets a
new patch release, and a `retract` line in `go.mod` if nobody should use it.
