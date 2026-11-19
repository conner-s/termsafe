# Using termsafe in another project

Copy this directory. It imports only the standard library and nothing from
the program it sits in, so it builds wherever it lands. Keep it identical
across projects: fix a bug here and copy it again.

glip is the worked example. `docs/untrusted-text.md` in that repository is
the table this method produces.

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

## The two tests

Both come from the kit in `testkit.go`.

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

Before trusting either test, see it fail: point the conversion at the raw
string once and run it.

A third kind of test is worth its fixtures: capture what the real tools
print with the program's own arguments, into a file, and replay the bytes
through the reader, the parser and the view. It is how glip found that
yt-dlp's progress never reached its progress bar.

## What termsafe does not do

- It does not find a name written wholly in look-alike letters, or
  look-alikes within one script. That needs Unicode's confusables table.
- It does not clean what a child process writes to a terminal it was given.
- It does not make text safe for anything but a terminal: not for a shell,
  HTML, a prompt or a file name.
