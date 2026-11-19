# Using termsafe in a program

termsafe is one Go package in its own module, with no dependencies. This
page says how to add it to a program, the method for finding every place it
is needed, the tests that prove the result, and what a repository that
vendors or builds with Nix has to do as well.

glip, the program the package was written in, is the worked example:
`docs/untrusted-text.md` in its repository is the table this method
produces, and `ui/hostile_test.go` holds the tests. nfu
(`github.com/conner-s/nfu`) is a second one, with a TUI and a plain output.

## Add the module

```sh
go get github.com/conner-s/termsafe
```

```go
import "github.com/conner-s/termsafe"
```

`go get` takes the newest tag and writes it to `go.mod` and `go.sum`. Commit
both. The module is public: it comes through the public module proxy and is
checked against the public checksum database, with no credentials, no extra
certificate authority and no `GOPRIVATE`. The GitHub repository it comes
from is a read-only mirror of the owner's forge, where the development
happens; `README.md` says what that means for a reader on GitHub.

A program that used v0.1.0 has it under another path,
`forgejo.kube.internal.conn3r.xyz/connerlab/termsafe`. That version does not
resolve under the new path, even if the mirror shows its tag. Change the import path in every file, `go get` this module,
and run `go mod tidy` to drop the old requirement; `CHANGELOG.md` lists what
v0.2.0 changed in behaviour. A program that still carries a copy of the
package from before it was a module deletes the copy and does the same.

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
- A program that owns the terminal may also ask it, before its TUI starts:
  `terminal = terminal.Probe(os.Stdin, os.Stdout, timeout)`, with the
  terminal in raw mode for that moment. `README.md`, "Asking the terminal",
  has the code and the conditions. The answer outranks the environment.
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

A view may show an escaped form on purpose: the TOML an encoder wrote, with
an ESC as the six characters `\u001B`, or a name quoted with `%q`. The
payload of a control string then stands after the escape as readable text,
and `ViewProblem` reports "the payload of a control sequence is drawn as
text", because from a cleaner that means half a sequence was removed. For
such a view that picture is the intended one, but only when the escape is
there to be read. Have the test allow exactly that finding, and only when
the drawn text holds the visible escape, and nothing else. nfu's
`hostile_test.go` has the helper.

A third kind of test is worth its fixtures: capture what the real tools
print with the program's own arguments, into a file, and replay the bytes
through the reader, the parser and the view. It is how glip found that
yt-dlp's progress never reached its progress bar.

## A repository that vendors

After `go get`, run `go mod vendor` and commit `go.mod`, `go.sum`,
`vendor/modules.txt` and `vendor/github.com/conner-s/termsafe/` together.

- `go mod vendor` copies `termsafe.go`, `inspect.go`, `testkit.go` and the
  files of the terminal check and the probe, so the program's tests build
  from the vendored copy, kit included.
- It leaves out the package's own tests. The vendored copy cannot be tested
  in place; the tests run in the termsafe repository.
- It also copies the files that lie beside the package: `README.md`,
  `PORTING.md`, `CHANGELOG.md` and `LICENSE`, which belong next to vendored
  code, and `.woodpecker.yaml` and `renovate.json`, which do nothing there:
  Woodpecker and Renovate read their files at the root of a repository
  only. Nothing from a subdirectory is copied, which is why termsafe keeps
  the mirror's workflow in `.github/`, its instructions for agents in
  `.claude/` and its maintainer's script in `scripts/`.
- Never edit the vendored copy. Fix termsafe, tag it, and take the tag.

## A repository that builds with Nix

Nothing special. termsafe is a public module like any other.

- With `vendorHash = null` and a committed `vendor/`: vendor as above; the
  Nix expression does not change.
- With a pinned `vendorHash`: after adding the requirement, refresh the
  hash. Set `vendorHash = lib.fakeHash`, build, and paste the `got:` hash.

## What termsafe does not do

- It does not find a name written wholly in look-alike letters, or
  look-alikes within one script. That needs Unicode's confusables table.
  `README.md` lists exactly what `MixedScripts` misses.
- It does not clean what a child process writes to a terminal it was given.
- It does not make text safe for anything but a terminal: not for a shell,
  HTML, a prompt or a file name.
