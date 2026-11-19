# Changelog

Every release of termsafe, newest first. "Changed" lists what can break a
program that imports it. `README.md` says what a v0 version number promises.

## Unreleased

Nothing yet.

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
