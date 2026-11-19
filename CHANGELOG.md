# Changelog

Every release of termsafe, newest first. "Changed" lists what can break a
program that imports it. `README.md` says what a v0 version number promises.

## Unreleased

Nothing yet.

## v0.2.0 - not tagged yet

Two things change what cleaning does, and the package stops assuming a UTF-8
terminal. Everything v0.1.0 exported is still there with the same signature.

### Changed

- **A variation selector that selects nothing is now an invisible
  character.** A selector (U+FE00 to U+FE0F, U+E0100 to U+E01EF, the
  Mongolian ones) is kept only directly after a character it can modify,
  one to a character. Any other, such as one after a Latin letter or the
  second of two, is removed under `Drop`, shown as `<U+FE0F>` under `Mark`,
  and counted in `Report.Invisible`. v0.1.0 let every selector through, so a
  run of them could carry hidden bytes. Emoji, keycaps, Han variants and the
  other sequences Unicode defines are untouched.
  Who it affects: every program. Text that held stray selectors now comes
  out shorter or marked, and a program that reports `Invisible` to its user
  will report these. In glip that is the warning about the user's own input;
  tool output loses them silently, as it does other invisible characters.
- **U+2028 and U+2029 are line breaks.** `Clean` makes each a newline and
  `Line` a space, as both already did with a carriage return. They are white
  space and the report does not count them. v0.1.0 passed both through, so
  a `Line` could hold a line break.
  Who it affects: a program that splits `Clean` output into lines gets one
  more line for each; a one-line field can no longer hold either.
- **`Samples()` returns 28 samples, four more**: "variation selector run",
  "variation selector after a Latin letter", "second variation selector" and
  "line and paragraph separators". The third starts with an emoji, U+2764,
  which stays in the cleaned text.
  Who it affects: every program that runs the samples through its views.
  The new ones run on the first test after the upgrade, with no change to
  the tests.
- **`ViewProblem` and `TerminalProblem` find more**: a line or paragraph
  separator, and two variation selectors in a row.
  Who it affects: a program whose own text holds either. None should.
- With the tables of Go 1.26 (Unicode 15.0), a selector after a character
  that Unicode added later is removed where Go 1.27 keeps it. `README.md`
  has the list under "Unicode versions".

### Added

- `CheckTerminal`, `Terminal` and `Encoding` (`EncodingUnknown`,
  `EncodingUTF8`, `EncodingNotUTF8`): what the locale variables and, on
  Windows, the console say about whether the terminal decodes UTF-8, with
  the reason and the evidence as text a program can show.
- `Text.ASCII`: the text with every character outside ASCII shown by its
  code point, for a terminal that cannot be taken as UTF-8. `Terminal.Fit`
  chooses between a text and its ASCII form.
- `ASCIIProblem` in the test kit: finds any byte outside ASCII in what a
  program drew for such a terminal.
  Who these affect: nobody until they are called. A program that draws a
  `Text` without calling `CheckTerminal` is assuming UTF-8, as every program
  did with v0.1.0. `README.md`, "Terminals that are not UTF-8", says what to
  add; `PORTING.md` has the steps and the third test.
- The Unicode version the tests ran under is printed by
  `TestUnicodeVersion`, and CI runs the tests on Go 1.26 as well as 1.27.
- `testdata/unicode` holds two data files of Unicode 17.0, under the Unicode
  License V3, for the test that holds the selector rule to Unicode's lists.

### Moved

- `CLAUDE.md` is now `.claude/CLAUDE.md`. `go mod vendor` copies every file
  beside the package into each program, and instructions for working on
  termsafe do not belong in another repository's tree.

## v0.1.0 - 2026-11-19

The first release. This is the package glip carried as `internal/termsafe`,
as it stood on glip's branch `deps-2026-11` at e3ef2ad and was reviewed in
connerlab/glip#12, moved into a module of its own. Its API and behaviour are
those of that copy.

### Added

- `Policy` with `Mark`, the zero value, and `Drop`. `Clean`, `Line`,
  `CleanReport` and `LineReport` clean a string under a policy.
- `Text`, which only cleaning can make, with `String`, `IsEmpty`,
  `TrimSpace`, `Truncate` and `MixedScripts`.
- `Report`, the counts of what cleaning took out, with `Changed` and
  `String`.
- `Mixed`, a word whose letters mix scripts that are not written together.
- The test kit: `Sample`, `Samples`, `SampleMarker`, `ViewProblem` and
  `TerminalProblem`.
- Runnable examples in `example_test.go`.

### Changed

- The import path, from a directory inside each program to
  `forgejo.kube.internal.conn3r.xyz/connerlab/termsafe`.
