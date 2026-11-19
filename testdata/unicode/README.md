# Unicode data files

Two files of the Unicode Character Database, version 17.0.0, unchanged:

- `StandardizedVariants.txt`, from
  <https://www.unicode.org/Public/17.0.0/ucd/StandardizedVariants.txt>
- `emoji-variation-sequences.txt`, from
  <https://www.unicode.org/Public/17.0.0/ucd/emoji/emoji-variation-sequences.txt>

They are Unicode's, not termsafe's, and are under the Unicode License V3 in
`LICENSE` beside them. Only `TestSelectorsAgainstUnicodeData` reads them: it
checks that no variation sequence Unicode defines is removed or marked. The
package itself does not use them, and `go mod vendor` does not copy this
directory into a program.

To move to a newer Unicode version, replace both files with the ones of that
version and run the tests under a Go release that carries it.
