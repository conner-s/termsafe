# Changelog

Every release of termsafe, newest first. "Changed" lists what can break a
program that imports it. `README.md` says what a v0 version number promises.

## Unreleased

Nothing yet.

## v0.2.0 - 2026-11-19

The module has a new path and a new home. Two things change what cleaning
does, and the package stops assuming a UTF-8 terminal. Everything v0.1.0
exported is still there with the same signature.

### Changed

- **The repository names no private host or address.** It is mirrored to a
  public one, and a released version is a module zip the Go proxy keeps for
  good. The history notes say "a private forge" without the hostname, test
  fixtures use documentation addresses, and `scripts/host-neutral.sh` fails
  the pipeline if a tracked file names one.
  Who it affects: nobody who uses the package; contributors run the script.
- **The module path changed; update imports.** The module is now
  `github.com/conner-s/termsafe`, so that a program using it builds
  anywhere without a private certificate authority. The repository on
  GitHub is a public, read-only mirror; development stays on the owner's
  forge. v0.1.0 was published under a module path on that forge and
  does not resolve under the new path, even where the mirror shows its tag:
  v0.2.0 is the first version here.
  Who it affects: every program that used v0.1.0. Change the import path in
  every file, `go get github.com/conner-s/termsafe`, and `go mod tidy` to
  drop the old requirement. A build no longer needs `GOPRIVATE` or the
  forge's root certificate for this module.

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
  `EncodingUTF8`, `EncodingNotUTF8`): what the environment says about
  whether the terminal decodes UTF-8, with the reason and the evidence as
  text a program can show. On Unix it reads the locale variables. On
  Windows it goes by the kind of handle: a console is UTF-8 because Go
  writes to it in UTF-16, an MSYS or Cygwin terminal goes by the locale its
  shell exports, a pipe or a file is not known. The Windows part is compiled
  and vetted and has never run.
- `Terminal.Probe`: asks the terminal itself, for a program that owns it and
  calls this. It writes a cursor position query, one two-byte character and
  a second query through the input and output the caller hands in, within a
  timeout, and blanks what it drew. Its answer outranks the environment's.
  It asks nothing unless both files are a terminal in raw mode. Linux on
  amd64 and arm64, and macOS; elsewhere it answers "cannot tell".
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
  Beside the forge's pipeline, which gates a pull request, a workflow on
  the GitHub mirror runs after each merge, with a Windows job and a macOS
  job. It is the first place the Windows half of the package runs.
- `scripts/unicode-data.sh` fetches Unicode's lists of variation sequences
  and runs the test that holds the selector rule to every one of them. The
  lists are not in the repository; the ordinary tests check a table of the
  41 kinds of sequence instead.

### Moved

- `CLAUDE.md` is now `.claude/CLAUDE.md`. `go mod vendor` copies every file
  beside the package into each program, and instructions for working on
  termsafe do not belong in another repository's tree.

## v0.1.0 - 2026-11-19

The first release. This is the package glip carried as `internal/termsafe`,
as it stood on glip's branch `deps-2026-11` at e3ef2ad, moved into a module
of its own. Its API and behaviour are
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

- The import path, from a directory inside each program to a module path
  on a private forge.
  That path ended with this version: see v0.2.0.
