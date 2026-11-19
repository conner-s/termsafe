//go:build !windows

package termsafe

import "os"

// examineOutput says whether out is a character device. A terminal is one,
// and so is /dev/null. The answer only goes into the evidence.
func examineOutput(out *os.File) outputFacts {
	if out == nil {
		return outputFacts{kind: outNone}
	}
	info, err := out.Stat()
	if err != nil {
		return outputFacts{kind: outNone}
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return outputFacts{kind: outTerminal}
	}
	return outputFacts{kind: outNotTerminal}
}
