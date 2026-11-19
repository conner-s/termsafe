# termsafe

A Go package that makes text a program did not write safe to draw in a terminal. glip, Triate, glab, nfu, npk and vigil import it, so a change here reaches all of them. The package comment in `termsafe.go` is the specification. `README.md` states the guarantees and names the test that holds each.

## Invariants

Every change keeps these five.

- A `Text` is only ever made by cleaning. Its field stays unexported, and every exported function that returns a `Text` gets it from `clean` or from another `Text`.
- A `Report` carries counts only, never a byte that was removed. A `Mixed` carries text that was already cleaned.
- Only `EncodingUTF8` lets a character outside ASCII through `Terminal.Fit`. Not UTF-8, cannot tell and the zero `Terminal` all give the ASCII form, which holds printable ASCII, newline and tab and no other byte.
- Standard library only. `go.mod` has no `require`; CI checks that the module graph is this module alone.
- Clean ASCII input allocates nothing, through `clean`, `Text.ASCII` and `Terminal.Fit`. `TestCleanInputAllocatesNothing` holds it there.

Coverage of the package stays at 100% of statements. CI fails below that, and CI is Linux. Code that only a real terminal reaches would break it, so `probe_unix.go` asks the system in straight lines and leaves every decision to `refusal` and `ask`, which scripted terminals reach on any platform.

Nothing here has run on Windows. `terminal_windows.go` only gathers facts with system calls; every decision is in `judge`, which is tested everywhere with the facts written out. Keep it that way, and say "compiled and vetted", never "tested", about Windows.

## Changing behaviour

A change to what cleaning removes, marks or keeps comes with all three:

- a table case with the exact output expected, under both policies
- a hostile sample in `Samples()` in `testkit.go` that the old code lets through. A sample holds `SampleMarker` and no space, tab or newline. The programs that import termsafe run every sample through their own views, so this is how the change gets tested in them.
- a line in `CHANGELOG.md` under "Unreleased" that says which programs it affects

## Unicode tables

What is a letter, a symbol, an ideograph or a format character comes from the `unicode` package, so it depends on the Go release: Unicode 15.0 in Go 1.26, 17.0 in Go 1.27. CI runs the tests on both. A test that needs a character added after 15.0 skips it when the tables do not know it, as `TestSelectorClasses` does. When the package comment quotes numbers about the tables, recount them before changing them.

Unicode's data files are not kept in this repository. After a change to `takesSelector`, and when a Go release brings a new Unicode version, run `scripts/unicode-data.sh`: it fetches the lists of variation sequences and checks every one. The ordinary tests only check the table of kinds in `selector_test.go`.

## Layout

`go mod vendor` copies every file that lies beside the package into each program that imports it, and nothing from subdirectories. So this file is in `.claude/` and the maintainer's script in `scripts/`. `README.md`, `PORTING.md`, `CHANGELOG.md` and `LICENSE` are at the root because a program wants them next to the vendored code. Put anything else in a subdirectory.

## Conventions

- Build every invisible or look-alike character from its code point with a name, `string(rune(0x202E))`, in code, tests and docs. A literal one hides in a diff. Agent tooling can turn a typed backslash-u escape into the character itself, so after writing a file run `LC_ALL=C grep -nP '[^\x00-\x7F]' <file>` and account for every hit.
- CI formats with the newest Go. gofmt of Go 1.27 aligns a table differently from gofmt of Go 1.26 when one key is much longer than the rest, so keep keys short, with a variable for a long one, and both agree.
- A pushed tag is never moved or deleted. A bad release gets a new patch release.
