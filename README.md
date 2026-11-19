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
import "github.com/conner-s/termsafe"
```

```sh
go get github.com/conner-s/termsafe
```

Fetching it takes no credentials, no extra certificate authority and no
`GOPRIVATE`: Go gets it through the public module proxy and checks it against
the public checksum database, like any other module.

**The GitHub repository is a read-only mirror.** termsafe is developed on
its owner's own forge, which is not reachable from outside: the commits,
the issues, the pull requests and the CI that gates them are there, and the
forge pushes `main` to GitHub. If you are reading this on GitHub:

- A pull request opened on GitHub will not be merged there, and the next
  push from the forge would not include it.
- There is no public place to report a problem yet. Contact: none yet.
- The "Mirror" workflow on GitHub runs after each push from the forge. It is
  where the package is tested on Windows and macOS.

Version 0.1.0 was published under another path,
`forgejo.kube.internal.conn3r.xyz/connerlab/termsafe`, on that forge. The
first version under this path is v0.2.0; a program that used v0.1.0 changes
its import path and nothing else, apart from what CHANGELOG.md lists for
v0.2.0. The mirror may show a tag `v0.1.0` all the same, because the forge
has it. Its `go.mod` declares the old path, so
`go get github.com/conner-s/termsafe@v0.1.0` fails with a complaint about
the module path. That is expected: v0.2.0 is the first version that
resolves.

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
func (c Terminal) Probe(in, out *os.File, timeout time.Duration) Terminal

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
| A variation selector that selects nothing is an invisible character like the others: one after a character it cannot modify, a second one in a row, a run. Every variation sequence Unicode 17.0 defines comes through untouched. | `TestStraySelectors`, `TestSelectorsOnTheirBases`, `TestSelectorClasses` (the kinds of sequence); `TestSelectorsAgainstUnicodeData` (every sequence, when a maintainer supplies Unicode's files) |
| `Terminal.Probe` writes nothing unless both files are a terminal in raw mode, never waits past its timeout, and leaves the screen as it was. | `TestAsk` and `TestProbeWith` (scripted terminals with a screen), `TestAsk_OverDescriptors` (real deadlines), `TestProbe_OnATerminal` (a pseudo terminal), `TestProbe_NotATerminal` |
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

- Nothing Unicode defines is rejected: all 1,353 standardized variation
  sequences, all 742 emoji variation sequences and all 39,501 sequences
  registered in the Ideographic Variation Database pass. Unicode's files are
  not in this repository. `scripts/unicode-data.sh` fetches them and runs
  the check over every sequence; a maintainer does that by hand. What runs
  on every push is a table of the 41 kinds of sequence the two Unicode files
  hold (selector, script, category), with up to three characters of each.
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
program has three things to do, and a fourth it may add.

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
4. If the program owns the terminal, it may ask the terminal itself:
   `terminal = terminal.Probe(os.Stdin, os.Stdout, timeout)`. That is never
   done unless the program calls it. [Asking the terminal](#asking-the-terminal)
   has the conditions.

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
- On Windows, what the file is. The table under
  [Windows](#windows) has every case.
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

### Windows

On Windows the verdict follows from how Go's `os` package writes to the
kind of handle the program was given. This is from the source of Go 1.26.10
and Go 1.27.3, which agree:

- `os.newFile` (`os/file_windows.go`) calls a handle a console when
  `syscall.GetConsoleMode` succeeds on it.
- For a console, `(*FD).Write` (`internal/poll/fd_windows.go`) calls
  `(*FD).writeConsole`, which decodes the bytes as UTF-8 and passes the
  characters to `syscall.WriteConsole`, which is `WriteConsoleW`, as UTF-16.
  No byte reaches the console, so no code page decodes one.
- For a pipe or a file, the same `Write` calls `syscall.WriteFile` with the
  bytes as they are.

| The output is | What Go writes | What the terminal receives | Verdict |
| --- | --- | --- | --- |
| A console with virtual terminal processing: Windows Terminal, a ConPTY under an editor's terminal, the classic console once a TUI framework has switched the mode on | UTF-16 characters, `WriteConsoleW` | the characters; it obeys escape sequences among them | `EncodingUTF8` |
| A console without virtual terminal processing | the same | the characters; it draws escape sequences as text | `EncodingUTF8` |
| A console under `sshd` (Win32-OpenSSH gives the shell a ConPTY) | the same | the console receives the characters and sends UTF-8 on to a terminal on another machine | `EncodingUTF8`, with the ssh session in the evidence |
| An MSYS or Cygwin terminal: mintty, Git Bash, the Cygwin terminal. Windows presents it as a pipe named like `msys-...-pty0-to-master` | the bytes, `WriteFile` | the bytes, decoded in the terminal's own charset, which MSYS and Cygwin export as the locale | by `LC_ALL`, `LC_CTYPE`, `LANG`, as on Unix |
| Any other pipe | the bytes | whatever the reader makes of them | by the locale variables; usually none is set, so `EncodingUnknown` |
| A file, or `NUL` | the bytes | nothing draws them yet | the same |

The code page is in the evidence for the console rows and decides nothing
there: 437, 932 and 65001 give the same verdict for the same reason. It does
decide for bytes that reach a console another way, a child process's output
for one, and for those only 65001 is UTF-8. That is outside what termsafe can
see.

The verdict for a console is a statement about `*os.File`. A program that
writes to the console handle some other way is not covered by it.

The whole table is one function, `judge`, which is given the facts and makes
no system call. It is tested on every platform, with the facts for each row
written out. The part that gathers the facts on Windows, four system calls in
`terminal_windows.go`, runs only in the Windows job on the mirror: see
[What was tested](#what-was-tested-and-what-was-not) for how far that goes.

### Asking the terminal

The environment can be wrong. The one way to find out is to ask the
terminal, and `Terminal.Probe` does, for a program that owns the terminal and
calls it:

```go
terminal := termsafe.CheckTerminal(os.Stdout)

// Before a TUI starts, the terminal is in the mode the shell left it in.
// Switch echo and line editing off for the moment of the question and put
// the mode back. The package for that is one the program has already,
// through its TUI framework: github.com/charmbracelet/x/term under
// bubbletea, golang.org/x/term elsewhere.
if state, err := term.MakeRaw(os.Stdin.Fd()); err == nil {
	terminal = terminal.Probe(os.Stdin, os.Stdout, 200*time.Millisecond)
	_ = term.Restore(os.Stdin.Fd(), state)
}

p := tea.NewProgram(newModel(terminal)) // then the program loop, as before
```

What it does: it writes the cursor position query `CSI 6 n` and reads the
answer. Then it writes one character of two UTF-8 bytes, U+00E4 (C3 A4), and
the query again. A terminal that decodes UTF-8 has moved one cell. An 8-bit
terminal has drawn two characters and moved two; a terminal in an East Asian
double-byte set has drawn one wide character and moved two. Then it blanks
what it drew and puts the cursor back. U+00E4 was chosen because both of its
bytes are printable in the 8-bit sets, neither is a C1 control, and Unicode
gives it a fixed narrow width, which U+00E9 does not have.

What the answer changes: it outranks the environment. `Encoding` becomes
what the terminal showed, `Reason` says so and names what the environment had
said, and `Evidence` keeps the environment's lines and gains one. With no
answer, `Encoding` and `Reason` stay as they were and the new line of
evidence says why.

What it will not do:

- It opens nothing and sets no terminal mode. The caller hands in the input
  and the output it already holds.
- It asks only when both are a terminal. Given a pipe, a file, the null
  device or nil it writes nothing and reads nothing.
- It asks only when the terminal is in raw mode, which it reads and does not
  set. In any other mode the terminal driver would draw the answer on the
  screen and keep it from the program until the next newline. So a program
  that forgets the `MakeRaw` above gets "cannot tell", not a mess.
- It never waits past the timeout. It waits with `select` and reads only
  what is there, so no read is left open for a later keypress to fall into.
- It draws nothing before the terminal has answered once, and it asks
  nothing when the cursor is within two cells of the right edge.

What a caller should still know. A key the user presses during the probe is
read and dropped. An answer that arrives after the timeout is still on its
way, and the program reads it later as input; a TUI framework takes it for
the cursor position report it is. The cell or two under the cursor are blank
afterwards, whatever they held. It is implemented for Linux on amd64 and
arm64 and for macOS; elsewhere it answers "cannot tell" without asking. On
Windows there is nothing to ask in the case that matters, the console.

Inside a running TUI the framework owns the input, and `Probe` must not be
called there. Ask before the loop starts, as above.

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

`TestUnicodeVersion` prints the version it ran under, and both CIs run the
tests on both Go releases, so the logs show both. Go 1.26 stays in them until
Go 1.28 is released, when it leaves Go's support.

## What was tested, and what was not

Tested in this repository, in two places.

Before a merge, on the forge: a Woodpecker pipeline on every pull request
and every push to `main`, on Linux, with Go 1.27 and with Go 1.26.
Everything in the list below runs there, and a pull request does not merge
without it.

After a merge, on the GitHub mirror: the "Mirror" workflow, on what the
forge pushed. It has the runners the forge lacks: a Windows job, a macOS
job, and a Linux job that repeats the pipeline's checks. **It runs after the
merge, not before it.** A failure there, a Windows failure most likely,
shows up on a `main` that already holds the change. Then: treat it as a bug
on `main`, fix it or revert it through a pull request on the forge, and tag
no release from a commit whose mirror run is not green.

- Tables of hostile input with the exact output expected: CSI, OSC, DCS,
  APC, PM and SOS in their ESC and C1 forms, terminated and not, cut by a
  newline or by another escape; every C0 and C1 control; raw 8-bit bytes
  and broken UTF-8; the Trojan Source shapes; invisible characters;
  variation selectors that select nothing; both policies; the report for
  each.
- A table of the 41 kinds of variation sequence Unicode 17.0 defines, with
  up to three characters of each kind: none is touched. An ordinary run does
  not prove that every defined sequence passes. Unicode's lists are not in
  the repository, so that it holds no third-party data. A sequence of a
  kind that is not in the table, in a future Unicode version, would go
  unnoticed until someone runs `scripts/unicode-data.sh`, which fetches the
  lists and checks every sequence, the 39,501 registered ideographic ones
  too. It was last run for v0.2.0, under both Go releases.
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
  as the value of a locale variable. The table for Windows is part of it
  and runs on Linux and macOS: it is given the facts a Windows machine
  would report.
- The probe against scripted terminals that keep a screen: one that decodes
  UTF-8, an 8-bit one, a double-byte one, one that never answers, one that
  answers with garbage, one that answers after the deadline, one that
  answers once. Each run ends with the screen as it was. The same over real
  pipes with real deadlines, and over a pseudo terminal, where the kernel's
  terminal driver sits between the probe and the scripted terminal: in raw
  mode it asks, in the mode a shell leaves it asks nothing.
- Statement coverage of the package is 100%, and the pipeline fails below
  that.

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
- **Windows, beyond what a CI runner can show.** Development is on macOS and
  the forge's CI is Linux. The Windows half of this package runs in one
  place, the Windows job of the mirror's workflow, after a merge. That job
  runs the whole suite and then the two tests written for Windows, and its
  last step says for each of the two whether it ran and passed, or skipped.
  Read its summary before reading "passed" into a green job.
  - `TestWindowsExamineOutput` needs no console. It looks at a pipe, a file
    and `NUL`, and can run on a runner.
  - `TestWindowsConsoleReceivesCharacters` is the evidence for the console
    rows of the table: it sets the console to code page 437, writes three
    characters through an `*os.File` and reads the cells back from the
    screen buffer. A runner has no console window. The test takes the
    console its process was given, asks Windows for one if it has none, and
    skips itself only if both fail; the job then says "NOT RUN, SKIPPED" and
    raises a warning. Either way it has not run in a real console window,
    Windows Terminal or the classic one. On a Windows machine:
    `go test -run Windows -v .`
  - Not covered by any of this: mintty and Git Bash, where the output is a
    pty that Windows presents as a pipe. That row is tested as a decision,
    with the facts written out, and never against the real thing.
  - Running the Windows build under Wine was tried on the development
    machine and could not be done: nixpkgs has no Wine for an Apple Silicon
    Mac, and its x86_64 build cannot start without Rosetta.
  - The workflow was written on 2026-11-19 and had not run when this was
    written. Its first run is the first time any of this code runs on
    Windows, and the Windows tests themselves were written without a
    Windows machine: a failure there may be the test's.
- The terminal check against real terminals in other encodings. Its table
  is what locales say, not what any terminal did.
- The probe against a real terminal emulator. Its terminals are scripted.
  The pseudo terminal test runs on Linux in both CIs and on macOS in the
  mirror's workflow, whose jobs fail if that test skipped itself. In the
  forge's pipeline a skip does not fail the run. Before the first of those
  runs it had run on macOS only, and the probe's Linux constants for the
  terminal mode are from the kernel's headers for amd64 and arm64, not from
  a run.
- Nobody outside the project has reviewed it. Its author's own review
  happened in the program it was written in, glip, before it became a
  module.

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

A release is made on the forge and has to reach GitHub, because GitHub is
where Go fetches it from.

1. CHANGELOG.md has a section for the version, on `main`.
2. The commit passed the forge's pipeline, and the mirror's workflow run for
   that commit is green.
3. An annotated tag `vMAJOR.MINOR.PATCH` goes on that commit, on the forge.
4. Confirm the tag is on GitHub. The forge mirrors `main`; whether it
   carries tags too is a setting, so look rather than assume, and push the
   tag there if it is missing:
   `git ls-remote --tags https://github.com/conner-s/termsafe`
5. Confirm Go resolves it: `go list -m github.com/conner-s/termsafe@vX.Y.Z`
   from outside any module.

A tag is never moved or deleted. The module proxy and the checksum database
keep the first contents they saw for a version, for good, and programs
record the checksum in their `go.sum`: a moved tag would differ from all
three. A bad release gets a new patch release, and a `retract` line in
`go.mod` if nobody should use it.

## Licence

MIT. See [LICENSE](LICENSE).
