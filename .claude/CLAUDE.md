# termsafe

A Go package that makes text a program did not write safe to draw in a terminal. glip, Triate, glab, nfu, npk and vigil import it, so a change here reaches all of them. The package comment in `termsafe.go` is the specification. `README.md` states the guarantees and names the test that holds each.

Where things happen:

- Development is on the owner's private forge: issues, pull requests, the Woodpecker pipeline in `.woodpecker.yaml` that gates a pull request, and Renovate. A change goes through a pull request there.
- The module path is `github.com/conner-s/termsafe`. The GitHub repository is a read-only mirror: the Forgejo pushes `main` to it. Nothing is committed, merged or reviewed on GitHub.
- `.github/workflows/mirror.yml` runs on the mirror after each push: Windows, macOS, and Linux again. It runs after the merge, so its failure means `main` is already wrong. Fix that through a pull request on the Forgejo, and tag no release from a commit whose mirror run is not green.
- A release is tagged on the Forgejo and must be on GitHub before Go can fetch it. Confirm the tag arrived (`git ls-remote --tags https://github.com/conner-s/termsafe`); do not assume the mirror carries tags.
- v0.1.0 was published under a module path on the private forge. The history notes in `README.md`, `PORTING.md` and `CHANGELOG.md` say so without naming the host, and nothing in this repository names an internal host or address: `scripts/host-neutral.sh` fails the pipeline if a tracked file does. The mirror, and every module zip the Go proxy keeps for good, is public.

## Invariants

Every change keeps these five.

- A `Text` is only ever made by cleaning. Its field stays unexported, and every exported function that returns a `Text` gets it from `clean` or from another `Text`.
- A `Report` carries counts only, never a byte that was removed. A `Mixed` carries text that was already cleaned.
- Only `EncodingUTF8` lets a character outside ASCII through `Terminal.Fit`. Not UTF-8, cannot tell and the zero `Terminal` all give the ASCII form, which holds printable ASCII, newline and tab and no other byte.
- Standard library only. `go.mod` has no `require`; CI checks that the module graph is this module alone.
- Clean ASCII input allocates nothing, through `clean`, `Text.ASCII` and `Terminal.Fit`. `TestCleanInputAllocatesNothing` holds it there.

Coverage of the package stays at 100% of statements. The pipeline fails below that, and so does the Linux job of the mirror's workflow. Code that only a real terminal reaches would break it, so `probe_unix.go` asks the system in straight lines and leaves every decision to `refusal` and `ask`, which scripted terminals reach on any platform.

The Windows half runs nowhere but in the Windows job of the mirror's workflow, after a merge. `terminal_windows.go` only gathers facts with system calls; every decision is in `judge`, which is tested everywhere with the facts written out. Keep it that way. The job's last step says which Windows tests ran and which skipped: a test that skips itself there must be reported there, and "passed on Windows" is said only of a test that step names as run.

## Changing behaviour

A change to what cleaning removes, marks or keeps comes with all three:

- a table case with the exact output expected, under both policies
- a hostile sample in `Samples()` in `testkit.go` that the old code lets through. A sample holds `SampleMarker` and no space, tab or newline. The programs that import termsafe run every sample through their own views, so this is how the change gets tested in them.
- a line in `CHANGELOG.md` under "Unreleased" that says which programs it affects

## Unicode tables

What is a letter, a symbol, an ideograph or a format character comes from the `unicode` package, so it depends on the Go release: Unicode 15.0 in Go 1.26, 17.0 in Go 1.27. CI runs the tests on both. A test that needs a character added after 15.0 skips it when the tables do not know it, as `TestSelectorClasses` does. When the package comment quotes numbers about the tables, recount them before changing them.

Unicode's data files are not kept in this repository. After a change to `takesSelector`, and when a Go release brings a new Unicode version, run `scripts/unicode-data.sh`: it fetches the lists of variation sequences and checks every one. The ordinary tests only check the table of kinds in `selector_test.go`.

## Layout

`go mod vendor` copies every file that lies beside the package into each program that imports it, and nothing from subdirectories. So this file is in `.claude/`, the mirror's workflow in `.github/` and the maintainer's script in `scripts/`. `.woodpecker.yaml` and `renovate.json` have to be at the root and are copied along; they do nothing in a vendor tree. `README.md`, `PORTING.md`, `CHANGELOG.md` and `LICENSE` are at the root because a program wants them next to the vendored code. Put anything else in a subdirectory.

## Conventions

- Build every invisible or look-alike character from its code point with a name, `string(rune(0x202E))`, in code, tests and docs. A literal one hides in a diff. Agent tooling can turn a typed backslash-u escape into the character itself, so after writing a file run `LC_ALL=C grep -nP '[^\x00-\x7F]' <file>` and account for every hit.
- The pipeline formats with the newest Go; the mirror's workflow checks with the gofmt of each release. gofmt of Go 1.27 aligns a table differently from gofmt of Go 1.26 when one key is much longer than the rest, so keep keys short, with a variable for a long one, and both agree.
- A pushed tag is never moved or deleted: the public module proxy and checksum database keep the first contents they saw. A bad release gets a new patch release.
- Actions in the mirror's workflow are pinned to a commit, with the release in a comment. The Forgejo's Renovate reads `.github/workflows` and proposes the next one, once this repository is in its list.
