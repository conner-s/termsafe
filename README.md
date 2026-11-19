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

All of that is about characters, and a terminal that does not decode UTF-8
reads the bytes of ordinary characters as controls. So the package also says
what it can find out about the terminal, and has an ASCII form of every text
for a terminal it cannot vouch for.

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
	// Once, at start: can the terminal be taken as UTF-8?
	terminal := termsafe.CheckTerminal(os.Stdout)
	if terminal.Encoding != termsafe.EncodingUTF8 {
		fmt.Fprintln(os.Stderr, "drawing ASCII only:", terminal.Reason)
	}

	// Fit returns the text as it is on a UTF-8 terminal and its ASCII
	// form on any other.
	drawTitle(terminal.Fit(termsafe.Mark.Line(hostileName))) // invoice<U+202E>fdp.exe
	drawTitle(terminal.Fit(termsafe.Drop.Line(hostileName))) // invoicefdp.exe

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
func (t Text) ASCII() Text                           // since v0.2.0

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

// The terminal, since v0.2.0.
type Encoding int
const (
	EncodingUnknown Encoding = iota // the zero value: cannot tell
	EncodingUTF8                    // UTF-8 as far as the environment says
	EncodingNotUTF8
)
func (e Encoding) String() string
type Terminal struct {
	Encoding Encoding
	Reason   Text
	Evidence []Text
}
func CheckTerminal(out *os.File) Terminal
func (c Terminal) Fit(t Text) Text

// The test kit, for a program's own tests.
type Sample struct{ Name, Text string }
const SampleMarker = "VISIBLE"
func Samples() []Sample
func ViewProblem(view string) string
func TerminalProblem(written string) string
func ASCIIProblem(written string) string             // since v0.2.0
```

## Guarantees

The right column names what holds each one: the type, or tests in this
repository.

| Guarantee | Held by |
| --- | --- |
| A `Text` with anything in it comes only from cleaning a string under a `Policy`, or from a method of another `Text`. A function that takes a `Text` cannot be handed a raw string. | The type: its one field is unexported. |
| A `Text` holds no ESC, no other C0 control, no DEL and no C1 control. `Clean` keeps newline and tab; `Line` turns them into spaces. With no ESC and no C1 in it, it holds no control sequence, and joining two cleaned texts cannot form one. | `TestClean`, `TestLine`, `TestInvariants_Random`, `TestSplitAcrossLines`, `FuzzClean` |
| A `Text` holds only valid UTF-8. Each invalid byte becomes U+FFFD. | the same |
| A `Text` holds no bidirectional control and no invisible format character, under either policy. `Drop` removes them, `Mark` replaces each with a marker such as `<U+202E>`. Three kinds stay because legitimate text needs them: a zero width joiner or non-joiner after a character it can act on, tag characters inside an emoji flag, and one variation selector directly after a character it can modify. | `TestFormatCharacters`, `TestNoFormatCharacterSurvives` (every code point), `TestBidiControlsAreTheTwelve` |
| A variation selector that selects nothing is an invisible character like the others: one after a character it cannot modify, a second one in a row, a run. Every variation sequence Unicode 17.0 defines comes through untouched. | `TestStraySelectors`, `TestSelectorsOnTheirBases`, `TestSelectorsAgainstUnicodeData` (Unicode's own lists) |
| A newline is the only line break in a `Text`, and a `Line` has none. A carriage return, U+2028 and U+2029 become a newline under `Clean` and a space under `Line`. | `TestLine_Whitespace`, `TestLineSeparators` |
| `Text.ASCII()` holds printable ASCII, and newline and tab where the text had them, and no other byte. `Terminal.Fit` returns that form for every terminal but one found to be UTF-8, the zero `Terminal` included. | `TestText_ASCII`, `TestSamplesThroughASCII`, `TestTerminal_Fit`, and `checkInvariants` |
| What `CheckTerminal` returns to show the user, `Reason` and `Evidence`, is one line of printable ASCII each, whatever the environment variables held. | `TestJudge_HostileEnvironment` |
| Cleaning a cleaned text changes nothing and reports nothing, under either policy. | `checkInvariants`, run by the tables, the random tests and the fuzz target |
| A control string without a terminator ends at the end of its line. Cleaning `a + "\n" + b` gives the cleaned `a`, a newline and the cleaned `b`, so it does not matter how a reader split its input into lines. The one exception is an `a` that ends in a carriage return, which forms one CR LF with the newline. | `TestInvariants_Random`, `TestSplitAcrossLines` |
| Printable text comes through byte for byte, including right-to-left text, emoji joined by zero width joiners, flags, and the joiners Persian and Indic scripts need. | `TestLegitimateTextIsUntouched` |
| A `Report` holds six counts and nothing else. No byte that was removed can leave through it. `Report.String()` is ASCII. | The type; `TestReport_String` |
| Input that needs no cleaning (printable ASCII, and for `Clean` newline and tab) is returned as the same string, with no allocation, through `ASCII` and `Fit` as well. | `TestCleanInputAllocatesNothing` |
| `MixedScripts` never changes the text. | `TestMixedScripts` |

Three of these have edges worth knowing.

Every row about what a `Text` holds is about characters. It is true on a
terminal that decodes UTF-8, and only there.
[Terminals that are not UTF-8](#terminals-that-are-not-utf-8) says why, and
what the ASCII form is for.

What is a format character, a symbol or an ideograph is defined by the
Unicode tables of the Go release the program is built with.
[Unicode versions](#unicode-versions) says what that changes.

`Truncate` counts code points. It never cuts inside one, but it can cut an
emoji sequence or part a combining mark from its letter, and it knows nothing
about how many columns a character takes. In the ASCII form every character
is one column, so there it is exact.

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

## Variation selectors

A variation selector draws nothing. It asks for another form of the
character before it: the emoji or the text form of a symbol, a variant of a
Han character, a rotated hieroglyph. A run of selectors after any character
is invisible too, and can carry hidden bytes: there are 256 selectors, a
byte each. Up to v0.1.0 termsafe let every selector through.

Now a selector is kept only when it stands directly after a character it can
modify, one to a character. Any other is removed under `Drop`, shown as
`<U+FE0F>` under `Mark`, and counted in `Report.Invisible`. It did not get a
count of its own in the report: it is an invisible character like the tag
characters the same field already counts, the words `Report.String()` uses
fit it, and a program that tells its user about `Invisible` needs no change.

The standard library has the category and the script of every character but
not Unicode's lists of defined sequences. So the rule goes by class of
character:

| Selector | Kept directly after |
| --- | --- |
| U+FE0E, U+FE0F (text, emoji) | a symbol (So, Sm) outside ASCII; `#`, `*`, `0` to `9`; U+203C, U+2049, U+2139, U+3030, U+303D |
| U+E0100 to U+E01EF | a unified ideograph |
| U+FE00 to U+FE02 | a unified ideograph; the quotation marks U+2018, U+2019, U+201C, U+201D |
| U+FE00, U+FE01 | eight ideographic and fullwidth punctuation marks; the mathematical script capitals |
| U+FE00 | a mathematical symbol (Sm) outside ASCII; `0` and the fullwidth zero; a character of Myanmar, Phags-pa or Manichaean |
| U+FE00 to U+FE06 | an Egyptian hieroglyph |
| U+180B to U+180D, U+180F | a Mongolian letter |
| U+FE07 to U+FE0D | nothing: no sequence uses them |

Measured against Unicode 17.0's data, with the tables of Go 1.27:

- Nothing Unicode defines is rejected. All 1,353 standardized variation
  sequences and all 742 emoji variation sequences pass, and a test reads
  them from Unicode's files on every run. All 39,501 sequences registered
  in the Ideographic Variation Database pass; that was checked once, by
  hand, and the file is not in the repository.
- Much that Unicode does not define is accepted, because a class is wider
  than a list. Either presentation selector on 8,439 characters, where 371
  have both presentations. Any of 240 ideographic selectors on 101,996
  ideographs, 24.5 million pairs, where 39,501 are registered and none uses
  a selector above U+E011F. U+FE00 to U+FE02 on every ideograph, where
  1,002 pairs are defined. Seven selectors on 5,073 hieroglyphs, where
  about 150 pairs are defined.
- With the tables of Go 1.26, 9 standardized sequences and 2 registered
  ideographic ones are rejected: their bases were added after Unicode 15.0.

Does the width matter? For hiding data it leaves one selector on each
character that could carry one. That is nothing at all in ASCII, Latin,
Greek or Cyrillic text, a choice of two after a symbol, and up to a byte per
ideograph in Chinese, Japanese or Korean text. Unicode has a renderer ignore
a selector it has no form for, so such a pair should draw as the plain
character. Closing the rest means carrying Unicode's lists
in the package, the registered ideographic sequences above all, and
regenerating them with each Unicode version.

A selector is judged by what stands before it in the result. Under `Drop`,
when an invisible character between a base and its selector is removed, the
selector meets its base and stays. Under `Mark` the marker stands between
them, and the selector is marked too.

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

- One variation selector on each character that could take one, whether or
  not Unicode defines that pair. [Variation selectors](#variation-selectors)
  counts them.
- Characters that draw as blank space: the no-break space, the spaces of
  other widths U+2000 to U+200A, the ideographic space U+3000, the braille
  blank U+2800.
- Private-use and unassigned code points. What a terminal draws for them
  depends on its font.

## Terminals that are not UTF-8

Every guarantee about what a `Text` holds is a statement about characters.
A terminal in an 8-bit mode does not see characters: it reads each of the
bytes 0x80 to 0x9F as a C1 control, and ordinary characters hold those bytes
in their UTF-8 encoding. U+00DB, a capital U with a circumflex, is C3 9B, and
9B is CSI. No choice of characters is safe there. Only ASCII is.

Up to v0.1.0 the package assumed UTF-8 and said so in a caveat. Now a
program has three things to do.

1. Call the check once, at start: `terminal := termsafe.CheckTerminal(os.Stdout)`,
   with the file it draws on.
2. Choose the path at each place a raw string becomes a `Text`:
   `terminal.Fit(termsafe.Drop.Line(raw))`. `Fit` returns the text as it is
   when the verdict is `EncodingUTF8`, and `Text.ASCII()` of it for
   `EncodingNotUTF8`, for `EncodingUnknown` and for a `Terminal` nobody
   filled in. The ASCII form shows every character outside ASCII by its
   code point, `caf<U+00E9>`.
3. Tell the user when the verdict is not UTF-8, once, with
   `terminal.Reason`, and give them a way to overrule the verdict in both
   directions. The check can be wrong either way.

Run `Text.MixedScripts` before the ASCII form, which has no letters of other
scripts left in it. `Mixed.String()` draws the word as it stands, so on such
a terminal show `Word.ASCII()` and the code points in `Odd`.

The program's own text needs the same care. Box borders, bullets, spinners
and progress bars are characters outside ASCII, and an 8-bit terminal reads
their bytes the same way: the full block U+2588 is E2 96 88. Draw ASCII ones
when the verdict is not UTF-8. `ASCIIProblem` in the test kit finds any that
were missed.

### What the check reads

- The locale, in the order a C library uses: `LC_ALL` if it is set and not
  empty, else `LC_CTYPE`, else `LANG`. A codeset of UTF-8 gives
  `EncodingUTF8`, however it is spelled, including macOS's bare
  `LC_CTYPE=UTF-8`. Any other codeset gives `EncodingNotUTF8`, and so do the
  `C` and `POSIX` locales, which are ASCII. A locale with no codeset, such
  as `de_DE`, gives `EncodingUnknown`: its default is Latin-1 on one system
  and UTF-8 on another. No locale at all gives `EncodingUnknown`.
- On Windows, whether the file is a console. Go's `os` package writes to a
  console with `WriteConsoleW`, in UTF-16, so the characters arrive whatever
  the console's output code page is. The verdict is `EncodingUTF8` and the
  code page goes into the evidence. This is true of writes through an
  `*os.File`. Bytes that reach the console another way, a child process's
  output for one, are decoded by the code page, and only 65001 is UTF-8.
  When the file is not a console (a pipe, a file, or mintty, which Windows
  presents as a pipe) the locale variables decide.
- Whether the file is a terminal at all, `SSH_CONNECTION` and `SSH_TTY`,
  `TMUX` and `STY`. These go into `Evidence` and do not change the verdict.

`Reason` is the one finding that decided, `Evidence` everything that was
looked at, in order. Both are `Text` and ASCII, so they can be drawn on the
terminal they are about. Values from the environment are cleaned and cut to
40 characters before they are quoted.

### What the check cannot know

- Whether the terminal emulator is set to the encoding the locale names.
  Nothing in the environment says. `EncodingUTF8` means UTF-8 as far as the
  environment says, and no more.
- Whether the terminal at the far end of an ssh session honours the locale
  of this end.
- What a multiplexer, a serial console or a recording tool in between does
  with the bytes.
- What will draw output that goes into a pipe or a file.
- Whether a terminal under `LANG=C` is in fact UTF-8. Many are: containers,
  cron jobs and `sudo` reset the locale. They get the ASCII form, which is
  safe and ugly, and the reason for the override in step 3.

### Asking the terminal

The one way to find out is to ask: write a carriage return, a character of
two UTF-8 bytes that are both printable in the 8-bit sets (U+00E9 is C3 A9),
then the cursor position query `CSI 6 n`, and read the reply. Column 2 means
the terminal drew one character and decodes UTF-8. Column 3 means it drew
two, or one double-width one, and does not. No reply within a moment means
it cannot be told. Then erase the line.

termsafe does not do this, on purpose. The reply arrives on the terminal's
input, which belongs to the program or its TUI framework, and two readers on
one input lose bytes to each other. Reading it also needs raw mode, which the
standard library reaches only through a different `ioctl` on every system.
And it writes to the terminal before the program has decided how to draw. A
program that owns the terminal through a framework can ask through the
framework: bubbletea v2 has `tea.Raw` to write the bytes and
`tea.RequestCursorPosition` for the query. It must never be the default, and
it must give up at once when the output is not a terminal. If that turns out
to be wanted in more than one program, the judging half (the bytes to write
and what a column means) could move here.

## Unicode versions

Which characters are letters, symbols, ideographs, format characters and
members of a script comes from the standard library's `unicode` package. The
tables are those of the Go release a program is built with: Unicode 15.0 in
Go 1.26 and Unicode 17.0 in Go 1.27, both read from `unicode.Version`. The
program's toolchain decides, not this module. glip built by nix-pkgs with Go
1.26 and the same glip built with Go 1.27 can clean one input differently.

Between 15.0 and 17.0 the sets of bidirectional controls, format characters,
default-ignorable characters and variation selectors are identical, and no
character of 15.0 changed in any property termsafe uses. Both were checked by
dumping the tables of both releases. What differs is the 10,615 characters
added in between. A program built with Go 1.26 does not know them:

| Input | Built with Go 1.26 | Built with Go 1.27 |
| --- | --- | --- |
| A variation selector after one of 4,938 new ideographs, 3,995 new hieroglyphs or 846 new symbols | removed or marked, and counted | kept |
| A word that holds one of 9,568 new letters, 11 of them Latin and 2 Cyrillic | `MixedScripts` splits the word there and does not count the letter | counted, and the word reported if it mixes scripts |
| A zero width joiner after one of the 13 new Latin and Cyrillic letters | kept | removed or marked, and counted |

A newer Go only ever knows more characters, and none loses a property. What
that does to one input goes both ways, as the table shows.

Three parts of the package depend on the tables: the variation selector
rule (categories So and Sm, the unified ideographs, and five scripts),
`MixedScripts` (letters and scripts) and the joiner rule (the Latin, Greek
and Cyrillic scripts). The control sequences, the control characters, the
ASCII form and the terminal check depend on none.

`TestUnicodeVersion` prints the version it ran under, and CI runs the tests
on both Go releases, so the log shows both.

## What was tested, and what was not

Tested in this repository, by CI on every push to `main` and every pull
request:

- Tables of hostile input with the exact output expected: CSI, OSC, DCS,
  APC, PM and SOS in their ESC and C1 forms, terminated and not, cut by a
  newline or by another escape; every C0 and C1 control; raw 8-bit bytes
  and broken UTF-8; the Trojan Source shapes; invisible characters;
  variation selectors that select nothing; both policies; the report for
  each.
- Every standardized and every emoji variation sequence of Unicode 17.0,
  read from Unicode's own files in `testdata/unicode`: none is touched.
- The tests run on Go 1.27 and on Go 1.26, whose Unicode tables differ.
- Every code point from U+0080 up, placed between two ASCII letters: none
  that is a bidirectional control or an invisible character survives.
- Two runs of 20,000 inputs glued from hostile fragments, with fixed seeds.
  One checks the guarantees above. The other checks that the report says
  something exactly when the text changed.
- A fuzz target over the same checks, seeded with every table and every
  sample, for 30 seconds per run. Nobody has run a long fuzzing campaign.
- The test kit against the cleaner. Cleaned, all 28 samples pass both
  checks. Raw, every sample fails the view check and all but three fail the
  terminal check: a renderer writes CSI sequences of its own, and a joiner
  or a single selector is right or wrong by what it follows, which is
  judged in the view.
- The ASCII form of every sample, cleaned either way under either policy:
  printable ASCII and nothing else. The random runs and the fuzz target
  check the same of every input.
- The terminal check over a table of locale settings, and with every sample
  as the value of a locale variable.
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
  Windows there is only `go vet` with `GOOS=windows`. The one file that is
  specific to Windows, the two system calls that ask whether the output is a
  console and for its code page, has never run. That Go writes to a console
  in UTF-16 was read in the source of Go 1.26 and 1.27, not watched on a
  Windows machine.
- The terminal check against real terminals in other encodings. Its table
  is what locales say, not what any terminal did.
- The method under "Asking the terminal". It is described, not implemented.
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
- A build with another Go release can clean one input differently, because
  the Unicode tables come from the standard library.
  [Unicode versions](#unicode-versions) lists how.
- v1.0.0 comes when the programs that use it have stopped asking for API
  changes.

## Releases

A release is an annotated git tag `vMAJOR.MINOR.PATCH` on a commit of `main`
that CI passed, made after CHANGELOG.md has a section for that version. Go
fetches the tag from this repository, so there is nothing to publish.

A tag is never moved or deleted. Programs record a version's checksum in
their `go.sum`, and a moved tag breaks their builds. A bad release gets a
new patch release, and a `retract` line in `go.mod` if nobody should use it.

## Licence

MIT. See [LICENSE](LICENSE).
