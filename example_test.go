package termsafe_test

import (
	"fmt"
	"os"

	"forgejo.kube.internal.conn3r.xyz/connerlab/termsafe"
)

// As in the package's tests, no invisible or look-alike character is written
// into this file: each is built from its code point.
const (
	rightToLeftOverride = string(rune(0x202E))
	greekUpsilon        = string(rune(0x03C5)) // looks like u
	eAcute              = string(rune(0x00E9))
)

// hostileName is a file name a remote site chose: a right-to-left override
// that turns "fdp.exe" around for the reader, and an OSC 52 sequence that
// would write to the clipboard.
const hostileName = "invoice" + rightToLeftOverride + "fdp.exe\x1b]52;c;aGk=\x07"

// drawTitle stands for a view. It takes a Text, so the compiler refuses a
// raw string: drawTitle("raw") does not build.
func drawTitle(title termsafe.Text) {
	fmt.Println(title)
}

func Example() {
	drawTitle(termsafe.Mark.Line(hostileName))
	drawTitle(termsafe.Drop.Line(hostileName))

	// Output:
	// invoice<U+202E>fdp.exe
	// invoicefdp.exe
}

func ExamplePolicy_LineReport() {
	text, report := termsafe.Drop.LineReport(hostileName)
	fmt.Println(text)
	if report.Changed() {
		fmt.Println("removed:", report)
	}

	// Output:
	// invoicefdp.exe
	// removed: 1 control sequence and 1 bidirectional control
}

func ExamplePolicy_Clean() {
	// Progress output redraws a line with a lone carriage return. Clean
	// keeps the states apart as lines; Line would join them with spaces.
	fmt.Println(termsafe.Drop.Clean("10%\r20%\x1b[2K\r\ndone"))

	// Output:
	// 10%
	// 20%
	// done
}

func ExampleText_ASCII() {
	title := termsafe.Drop.Line("caf" + eAcute + " " + hostileName)
	fmt.Println(title.ASCII())

	// Output:
	// caf<U+00E9> invoicefdp.exe
}

func ExampleTerminal_Fit() {
	// At start a program asks once: terminal := termsafe.CheckTerminal(os.Stdout).
	// Here the answer is made up, as if LANG were C.
	terminal := termsafe.Terminal{Encoding: termsafe.EncodingNotUTF8}

	title := termsafe.Drop.Line("caf" + eAcute)
	drawTitle(terminal.Fit(title))
	if terminal.Encoding != termsafe.EncodingUTF8 {
		fmt.Println("the terminal is", terminal.Encoding)
	}

	// Output:
	// caf<U+00E9>
	// the terminal is not UTF-8
}

func ExampleCheckTerminal() {
	terminal := termsafe.CheckTerminal(os.Stdout)
	if terminal.Encoding != termsafe.EncodingUTF8 {
		// Say it once, in ASCII: Reason and Evidence are.
		fmt.Fprintln(os.Stderr, "drawing ASCII only:", terminal.Reason)
	}
	// terminal.Evidence lists everything the check looked at, for a verbose
	// mode or a diagnostics screen.
}

func ExampleText_MixedScripts() {
	name := termsafe.Mark.Line("pip install req" + greekUpsilon + "ests")
	for _, m := range name.MixedScripts() {
		fmt.Printf("the word at byte %d mixes %s and %s letters; the odd one is %U\n",
			m.Start, m.Scripts[0], m.Scripts[1], m.Odd[0])
	}

	// Output:
	// the word at byte 12 mixes Greek and Latin letters; the odd one is U+03C5
}
