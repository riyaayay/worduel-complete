package trie

import (
	_ "embed"
	"strings"
)

// wordlistTxt is a newline-separated, upper-cased dictionary (derived from
// the standard American English word list, filtered to alphabetic-only
// entries of length 2-15) embedded directly into the server binary. Swap
// this for SCOWL/ENABLE or any other list without changing any other code.
//
//go:embed wordlist.txt
var wordlistTxt string

// LoadDefaultDictionary builds a Trie pre-loaded with the embedded
// dictionary. This is what main.go calls at startup.
func LoadDefaultDictionary() *Trie {
	t := New()
	lines := strings.Split(wordlistTxt, "\n")
	words := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			words = append(words, l)
		}
	}
	t.Load(words)
	return t
}
