# termsafe

A Go package that makes text a program did not write safe to draw in a terminal. glip, Triate, glab, nfu, npk and vigil import it, so a change here reaches all of them. The package comment in `termsafe.go` is the specification. `README.md` states the guarantees and names the test that holds each.

## Invariants

Every change keeps these four.

- A `Text` is only ever made by cleaning. Its field stays unexported, and every exported function that returns a `Text` gets it from `clean` or from another `Text`.
- A `Report` carries counts only, never a byte that was removed. A `Mixed` carries text that was already cleaned.
- Standard library only. `go.mod` has no `require`; CI checks that the module graph is this module alone.
- Clean ASCII input allocates nothing. `clean` returns its argument before it builds anything, and `TestCleanInputAllocatesNothing` holds it there.

Coverage of the package stays at 100% of statements. CI fails below that.

## Changing behaviour

A change to what cleaning removes, marks or keeps comes with all three:

- a table case with the exact output expected, under both policies
- a hostile sample in `Samples()` in `testkit.go` that the old code lets through. A sample holds `SampleMarker` and no white space. The programs that import termsafe run every sample through their own views, so this is how the change gets tested in them.
- a line in `CHANGELOG.md` under "Unreleased"

## Conventions

- Build every invisible or look-alike character from its code point with a name, `string(rune(0x202E))`, in code, tests and docs. A literal one hides in a diff. Agent tooling can turn a typed backslash-u escape into the character itself, so after writing a file run `LC_ALL=C grep -nP '[^\x00-\x7F]' <file>` and account for every hit.
- A pushed tag is never moved or deleted. A bad release gets a new patch release.
