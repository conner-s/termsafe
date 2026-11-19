package termsafe

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A Mixed is a word whose letters come from scripts that do not belong
// together: the mark of a name made to look like another, such as a Latin
// word with a Greek upsilon in place of its u.
type Mixed struct {
	// Word is the word, as it stands in the text.
	Word Text
	// Start is the byte offset of the word in the text's String.
	Start int
	// Scripts are the scripts of its letters, by the names of the unicode
	// package's tables, sorted: "Greek", "Latin".
	Scripts []string
	// Odd are the letters that are not of the word's main script, in the
	// order they stand. The main script is the one most of its letters
	// have; when two have as many, it is Latin if that is one of them.
	Odd []rune
}

// String describes the finding, naming the odd letters by code point so they
// can be told from the letters they resemble:
//
//	"requests" mixes Greek and Latin letters (U+03C5 is Greek)
//
// The word is drawn as it is, so in a terminal it looks like the name it
// imitates; the code points are what tells them apart.
func (m Mixed) String() string {
	var odd []string
	for _, r := range m.Odd {
		odd = append(odd, fmt.Sprintf("U+%04X is %s", r, scriptOf(r)))
	}
	return fmt.Sprintf("%q mixes %s letters (%s)", m.Word.s, joinAnd(m.Scripts), strings.Join(odd, ", "))
}

// MixedScripts returns the words of t whose letters mix scripts, in the order
// they stand. It reports and changes nothing.
//
// A word is a run of letters, combining marks and digits. Its scripts are
// those of its letters; digits, marks and characters Unicode gives no script
// do not count. A word is reported when it has letters of more than one
// script, unless the scripts are ones that are written together: Latin with
// Han, Hiragana and Katakana (Japanese), with Han and Hangul (Korean) or with
// Han and Bopomofo (Chinese).
//
// It is meant for names: a package, a host, a repository, a branch, a
// version. In running prose it will also report honest words such as a Greek
// mu before a Latin unit. What it cannot see is listed in the package
// comment.
func (t Text) MixedScripts() []Mixed {
	// ASCII has nothing to mix.
	ascii := true
	for i := 0; i < len(t.s); i++ {
		if t.s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return nil
	}

	var found []Mixed
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		if m, ok := inspectWord(t.s[start:end], start); ok {
			found = append(found, m)
		}
		start = -1
	}
	for i, r := range t.s {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsDigit(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		flush(i)
	}
	flush(len(t.s))
	return found
}

func inspectWord(word string, start int) (Mixed, bool) {
	count := map[string]int{}
	for _, r := range word {
		if !unicode.IsLetter(r) {
			continue
		}
		if s := scriptOf(r); s != "" {
			count[s]++
		}
	}
	if len(count) < 2 || writtenTogether(count) {
		return Mixed{}, false
	}

	scripts := make([]string, 0, len(count))
	for s := range count {
		scripts = append(scripts, s)
	}
	sort.Strings(scripts)
	// The main script is the one with most letters. Of two with as many it
	// is Latin, the script names are usually imitated in, and otherwise
	// the first by name, so the result does not depend on map order.
	main := scripts[0]
	for _, s := range scripts[1:] {
		if count[s] > count[main] || (count[s] == count[main] && s == "Latin") {
			main = s
		}
	}
	var odd []rune
	for _, r := range word {
		if !unicode.IsLetter(r) {
			continue
		}
		if s := scriptOf(r); s != "" && s != main {
			odd = append(odd, r)
		}
	}
	return Mixed{Word: Text{word}, Start: start, Scripts: scripts, Odd: odd}, true
}

// writtenTogether reports whether the scripts counted are a set that one
// writing system uses side by side. These are the combinations Unicode's
// security mechanisms (UTS #39, "highly restrictive") let pass.
func writtenTogether(count map[string]int) bool {
	for _, set := range [][]string{
		{"Latin", "Han", "Hiragana", "Katakana"},
		{"Latin", "Han", "Hangul"},
		{"Latin", "Han", "Bopomofo"},
	} {
		in := 0
		for _, s := range set {
			if count[s] > 0 {
				in++
			}
		}
		if in == len(count) {
			return true
		}
	}
	return false
}

// commonScripts are tried first; the rest of the unicode package's tables
// are searched in name order.
var commonScripts = []string{"Latin", "Cyrillic", "Greek", "Han", "Hiragana", "Katakana", "Hangul", "Arabic", "Hebrew"}

var otherScripts = func() []string {
	skip := map[string]bool{"Common": true, "Inherited": true}
	for _, s := range commonScripts {
		skip[s] = true
	}
	var names []string
	for name := range unicode.Scripts {
		if !skip[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}()

// scriptOf returns the name of r's script, or "" when Unicode gives it none
// of its own (Common, Inherited).
func scriptOf(r rune) string {
	for _, name := range commonScripts {
		if unicode.Is(unicode.Scripts[name], r) {
			return name
		}
	}
	for _, name := range otherScripts {
		if unicode.Is(unicode.Scripts[name], r) {
			return name
		}
	}
	return ""
}

func joinAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}
