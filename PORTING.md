# Using termsafe in a program

termsafe is one Go package in its own module, with no dependencies. This
page says how to add it to a program, the method for finding every place it
is needed, the tests that prove the result, and what a repository that
vendors or builds with Nix has to do as well.

glip is the worked example. `docs/untrusted-text.md` in that repository is
the table this method produces, and `ui/hostile_test.go` holds the two tests.

## Add the module

```sh
export GOPRIVATE=forgejo.kube.internal.conn3r.xyz
go get forgejo.kube.internal.conn3r.xyz/connerlab/termsafe
```

```go
import "forgejo.kube.internal.conn3r.xyz/connerlab/termsafe"
```

`go get` takes the newest tag and writes it to `go.mod` and `go.sum`. Commit
both.

The Forgejo is not on the internet, so the public module proxy and checksum
database cannot see it. `GOPRIVATE` makes Go skip them and fetch with git
from the Forgejo itself. Without it the go command sends the module's path
to proxy.golang.org first and to sum.golang.org to check the download, and
the second fails. Both were seen with the two services pointed at a dead
local port. Go asks the Forgejo over HTTPS where the repository
is, so the machine has to trust the connerlabPKI root certificate. The
repository is public, so git can clone it over HTTPS without a login where
git trusts that certificate too. Where git should clone over ssh instead, put
a rewrite in the environment of the command:

```sh
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=url.ssh://git@forgejo.kube.internal.conn3r.xyz/.insteadOf
export GIT_CONFIG_VALUE_0=https://forgejo.kube.internal.conn3r.xyz/
```

A program that carries a copy of the package from before it was a module
(glip had `internal/termsafe`) deletes the copy and changes the import path.
Nothing else changes. v0.1.0 has the API and the behaviour of that copy.

To take a later version, read `CHANGELOG.md` for it, `go get` the tag, and
run the tests below. A release that removes more than the one before can
change what a test of yours expects to see drawn.

## The method

### 1. List the sources

Everything the program shows that it did not write:

- output of every subprocess, both streams
- responses from the network: titles, descriptions, names, error bodies
- file names and file contents, including the program's own config and
  cache, which someone else may have written
- what a model wrote
- the user's input: arguments, the clipboard, pasted and typed text
- errors, which quote any of the above

Follow each to the struct field that carries it. An error is a carrier too.

### 2. List the sinks

Everything that reaches a terminal:

- the view of every TUI model, and each helper that renders a field
- every direct print: `fmt.Print*`, `log`, writes to `os.Stderr`, the last
  error printed by `main`
- the plain and `--json` modes of the same commands
- what the CLI library prints itself (usage, flag errors)
- panics
- children that are given the terminal (inherited stdio, `tea.ExecProcess`):
  the program does not stand between them and the screen, and cannot clean
  what they write. Say so; do not pretend to cover it.

### 3. One conversion per surface

For each surface (the TUI, the plain output), pick the one place where
outside text is wrapped for display, and convert there:

- Make every view setter, message field and render-helper parameter that
  carries outside text a `termsafe.Text`. The compiler then lists every call
  site, and a new one cannot pass a raw string.
- Convert where the message is built from the raw data, not in the view and
  not at each render site. Clean before the program's own styling is
  applied: a styled string holds the program's own colour sequences and can
  no longer be cleaned.
- Use `Line` for anything that must stay one line (a row, a name, a path, a
  version) and `Clean` for text with lines (an error, release notes, a diff).
  Clean before splitting into lines: a lone carriage return becomes a
  newline.
- Read subprocess output with a splitter that ends a line at `\n`, `\r\n`
  or a lone `\r`. Tools redraw progress with a lone `\r`; split on `\n` only,
  a whole run's progress arrives as one line.
- If the view truncates, truncate the `Text` (`Text.Truncate`), never the
  bytes of its string: half a character is a raw byte.

Where a view reads a domain struct that is also used for I/O, do not change
the struct. Build a display row from it once, when the data arrives, with
`Text` fields, and have the view read the row.

### 4. Choose the policy, per kind of text

Every call names what happens to bidirectional controls and invisible
characters:

- `termsafe.Drop`: removed. For logs and progress, and for text the program
  goes on to use, such as a URL.
- `termsafe.Mark`: each shown as `<U+202E>`. For text a person bases a
  decision on: a title, a package name, a version, a diff, a model's verdict.
  The reader sees that something was there.

Use the report (`CleanReport`, `LineReport`) wherever the program should say
that it changed something, and always for the user's own input.

For names that decide something (a dependency, a repository, a host), also
run `Text.MixedScripts` and flag what it finds next to the name. It never
changes the name. Where the name must be ASCII by the rules of its registry,
test for that as well: it is stricter.

### 5. Keep the raw value for everything that is not display

- paths that are opened, written or handed to a subprocess
- arguments of subprocesses (end the options with `--` before a value that
  came from outside)
- URLs that are fetched or opened
- values sent back to an API, and keys used for lookups and comparisons
- text sent to a model
- the clipboard
- `--json` output, which is data for another program
- whatever a parser reads

A `Text` is for the screen. `Text.String()` is not the original.

If the user's own input is cleaned and then used (a URL), tell the user:
that is what the report is for.

### 6. Ask about the terminal, once

Everything a `Text` promises is true on a terminal that decodes UTF-8. On
any other, the bytes of ordinary characters are controls, and only ASCII is
safe. `README.md` has the reasons under "Terminals that are not UTF-8".

- At start, call `termsafe.CheckTerminal` with the file the program draws
  on, and keep the result where the conversions of step 3 can reach it.
- At each conversion, pass the cleaned text through it:
  `terminal.Fit(termsafe.Drop.Line(raw))`. On a UTF-8 terminal that is the
  text itself. On any other it is the ASCII form, `caf<U+00E9>`.
- Give the user a way to overrule the verdict in both directions, a flag or
  a setting, and build the `Terminal` from it:
  `termsafe.Terminal{Encoding: termsafe.EncodingUTF8}`. The check reads the
  locale; it cannot see how the terminal emulator is set.
- When the verdict is not UTF-8, say so once, with `terminal.Reason`, and
  say how to overrule it.
- Give the program's own glyphs ASCII stand-ins for that case: borders,
  bullets, spinners, progress bars. Their bytes are read the same way.
- Truncate after `Fit`: in the ASCII form one character is one column.
  Run `MixedScripts` before it.

## The tests

Write them in the program, with the kit the package exports: `Samples`,
`SampleMarker`, `ViewProblem`, `TerminalProblem` and `ASCIIProblem`. Copy glip's
`ui/hostile_test.go` and replace its table of entries with the program's own.

1. **The view test.** Make a table of every way outside text gets into the
   program (one entry per source from step 1, each building the state a real
   run would reach with a given text in that position). For every entry and
   every `termsafe.Samples()` sample, render the view and require
   `termsafe.ViewProblem(view) == ""`.

2. **The terminal test.** For the same table, run the real program with its
   input disabled and a buffer as its output, wait until
   `termsafe.SampleMarker` has been drawn, quit, and require
   `termsafe.TerminalProblem(written) == ""`. This is the test that fails
   when a framework passes sequences through: it reads what the terminal
   would have received.

With bubbletea v2 the second test is:

```go
out := &syncBuffer{} // a bytes.Buffer behind a mutex
p := tea.NewProgram(model,
	tea.WithInput(nil),
	tea.WithOutput(out),
	tea.WithoutSignals(),
	tea.WithWindowSize(120, 40),
	tea.WithEnvironment([]string{"TERM=xterm-256color"}),
)
done := make(chan error, 1)
go func() { _, err := p.Run(); done <- err }()
for !strings.Contains(out.String(), termsafe.SampleMarker) {
	time.Sleep(5 * time.Millisecond) // with a deadline
}
p.Quit()
<-done
if problem := termsafe.TerminalProblem(out.String()); problem != "" {
	t.Error(problem)
}
```

3. **The same two, for a terminal that is not UTF-8.** Build the program's
   state with `termsafe.Terminal{Encoding: termsafe.EncodingNotUTF8}` in
   place of the check's result, and run both tests again. Besides
   `ViewProblem` and `TerminalProblem`, require
   `termsafe.ASCIIProblem(view) == ""` and
   `termsafe.ASCIIProblem(written) == ""`: not one byte outside ASCII, in
   the outside text or in the program's own drawing. This is the test that
   finds the border or the spinner that was left as it was.

Before trusting any of them, see it fail: point the conversion at the raw
string once and run it. For the third, leave out one `Fit`.

All of them iterate over `termsafe.Samples()`, so a new version of the module
that adds a sample runs it through the program without a change to the
tests.

A third kind of test is worth its fixtures: capture what the real tools
print with the program's own arguments, into a file, and replay the bytes
through the reader, the parser and the view. It is how glip found that
yt-dlp's progress never reached its progress bar.

## A repository that vendors

After `go get`, run `go mod vendor` and commit `go.mod`, `go.sum`,
`vendor/modules.txt` and
`vendor/forgejo.kube.internal.conn3r.xyz/connerlab/termsafe/` together.

- Only `go get` and `go mod vendor` need the Forgejo. Builds, tests and CI
  read `vendor/` and need neither the network nor `GOPRIVATE`.
- `go mod vendor` copies `termsafe.go`, `inspect.go` and `testkit.go`, so
  the program's two tests build from the vendored copy, kit included.
- It leaves out the package's own tests. The vendored copy cannot be tested
  in place; the tests run in the termsafe repository.
- It also copies the files that lie beside the package: `README.md`,
  `PORTING.md`, `CHANGELOG.md`, `LICENSE`, the CI file and `renovate.json`.
  The first four belong next to vendored code. The last two do nothing
  there: Woodpecker and Renovate read their files at the root of a
  repository only, and Renovate's own list of the files it found in glip
  shows it passing over a vendored CI file. Nothing from a subdirectory is
  copied, which is why termsafe keeps its instructions for agents in
  `.claude/` and its test data in `testdata/`.
- Never edit the vendored copy. Fix termsafe, tag it, and take the tag.

## A repository that builds with Nix

With `vendorHash = null` and a committed `vendor/`, as glip is built in
nix-pkgs, the Nix expression does not change. Vendor as above.

With a pinned `vendorHash`, `buildGoModule` builds the vendor directory
itself: a fixed-output derivation runs `go mod vendor` with nixpkgs' git and
nixpkgs' bundle of public certificate authorities. Two things change.

1. The hash. Set `vendorHash = lib.fakeHash`, build, and paste the `got:`
   hash, as after any change to `go.sum`.
2. That derivation now has to download termsafe from the Forgejo. Expect it
   to fail as it stands: the bundle does not hold the connerlabPKI root, and
   the public proxy does not have the module. Two halves of that have been
   seen. git, given only that bundle the way `buildGoModule` gives it,
   refuses the Forgejo's certificate ("unable to get local issuer
   certificate"). And Go in Renovate's container, which has the public
   authorities only, fails on another module of this Forgejo with "x509:
   certificate signed by unknown authority". The Nix build itself has not
   been run.

Three ways to deal with the second point, the first two already in use in
these repositories:

- Commit `vendor/` and set `vendorHash = null`. This is what glip does.
- Take termsafe as a flake input (`flake = false`) pinned to the tag, and in
  `postPatch` replace the module with that input, so `go mod vendor` copies
  it from the Nix store and never asks the Forgejo. Triate's `flake.nix` does
  this for the vigil module and has the working form, including why the
  replace goes through a relative symlink. The version is then pinned twice,
  in `go.mod` and in `flake.lock`; move both together.
- Give the vendor derivation what it lacks through `overrideModAttrs`: a
  certificate bundle that includes the connerlabPKI root, and `GOPRIVATE`.
  The builder must also be able to reach the Forgejo. Untried.

## What termsafe does not do

- It does not find a name written wholly in look-alike letters, or
  look-alikes within one script. That needs Unicode's confusables table.
  `README.md` lists exactly what `MixedScripts` misses.
- It does not clean what a child process writes to a terminal it was given.
- It does not make text safe for anything but a terminal: not for a shell,
  HTML, a prompt or a file name.
