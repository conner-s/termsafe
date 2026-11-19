//go:build !windows

package termsafe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOutputKind(t *testing.T) {
	if kind, _ := outputKind(nil); kind != outNone {
		t.Errorf("nil: kind %d, want outNone", kind)
	}

	file, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	if kind, _ := outputKind(file); kind != outNotTerminal {
		t.Errorf("a file: kind %d, want outNotTerminal", kind)
	}
	file.Close()
	if kind, _ := outputKind(file); kind != outNone {
		t.Errorf("a closed file: kind %d, want outNone", kind)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if kind, _ := outputKind(w); kind != outNotTerminal {
		t.Errorf("a pipe: kind %d, want outNotTerminal", kind)
	}

	// /dev/null is a character device, which is all outputKind can see of a
	// terminal.
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	if kind, _ := outputKind(null); kind != outTerminal {
		t.Errorf("%s: kind %d, want outTerminal", os.DevNull, kind)
	}
}

// TestCheckTerminal reads the real environment of the test process.
func TestCheckTerminal(t *testing.T) {
	for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG", "SSH_CONNECTION", "SSH_TTY", "TMUX", "STY"} {
		t.Setenv(name, "")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	got := CheckTerminal(w)
	if got.Encoding != EncodingUnknown {
		t.Errorf("nothing set: Encoding = %v, want cannot tell", got.Encoding)
	}
	checkShowable(t, got)

	t.Setenv("LANG", "en_US.ISO-8859-1")
	if got := CheckTerminal(w); got.Encoding != EncodingNotUTF8 {
		t.Errorf("LANG=en_US.ISO-8859-1: Encoding = %v, want not UTF-8", got.Encoding)
	}
	t.Setenv("LC_CTYPE", "en_US.UTF-8")
	got = CheckTerminal(w)
	if got.Encoding != EncodingUTF8 {
		t.Errorf("LC_CTYPE=en_US.UTF-8: Encoding = %v, want UTF-8", got.Encoding)
	}
	if want := "the output is not a terminal but a pipe or a file, and what will draw it is not known"; got.Evidence[0].String() != want {
		t.Errorf("first evidence = %q, want %q", got.Evidence[0].String(), want)
	}
	if got := CheckTerminal(nil); got.Evidence[0].String() != "the output could not be examined" {
		t.Errorf("nil: first evidence = %q", got.Evidence[0].String())
	}
}
