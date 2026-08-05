// Package trie implements a standard prefix trie used for real-time word
// validation in WordDuel. Lookups are O(k) where k is the length of the
// word being checked, regardless of how large the loaded dictionary is.
package trie

import "strings"

// Node is a single trie node. Children are keyed by rune so the trie works
// for any alphabet, not just a-z.
type Node struct {
	children map[rune]*Node
	isEnd    bool
}

func newNode() *Node {
	return &Node{children: make(map[rune]*Node)}
}

// Trie is a thread-safe-by-construction trie: it is built once at startup
// (Load) and only ever read afterward (IsValidWord / WordsWithPrefix), so no
// locking is required for concurrent reads across many goroutines.
type Trie struct {
	root *Node
	size int
}

// New returns an empty trie.
func New() *Trie {
	return &Trie{root: newNode()}
}

// Insert adds a single word to the trie.
func (t *Trie) Insert(word string) {
	word = strings.ToUpper(strings.TrimSpace(word))
	if word == "" {
		return
	}
	node := t.root
	for _, r := range word {
		child, ok := node.children[r]
		if !ok {
			child = newNode()
			node.children[r] = child
		}
		node = child
	}
	if !node.isEnd {
		node.isEnd = true
		t.size++
	}
}

// Load bulk-inserts a slice of words. Typically called once at server
// startup with a dictionary word list (e.g. ENABLE or SCOWL).
func (t *Trie) Load(words []string) {
	for _, w := range words {
		t.Insert(w)
	}
}

// Size returns the number of complete words stored in the trie.
func (t *Trie) Size() int {
	return t.size
}

// IsValidWord reports whether word exists in the dictionary. O(k).
func (t *Trie) IsValidWord(word string) bool {
	word = strings.ToUpper(strings.TrimSpace(word))
	node := t.walk(word)
	return node != nil && node.isEnd
}

// HasPrefix reports whether any word in the trie starts with prefix.
func (t *Trie) HasPrefix(prefix string) bool {
	return t.walk(prefix) != nil
}

// WordsWithPrefix returns every word in the trie starting with prefix, via a
// DFS from the prefix's terminal node. Powers a "hint" feature client-side.
func (t *Trie) WordsWithPrefix(prefix string) []string {
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	node := t.walk(prefix)
	if node == nil {
		return nil
	}
	var results []string
	var dfs func(n *Node, path string)
	dfs = func(n *Node, path string) {
		if n.isEnd {
			results = append(results, path)
		}
		for r, child := range n.children {
			dfs(child, path+string(r))
		}
	}
	dfs(node, prefix)
	return results
}

func (t *Trie) walk(s string) *Node {
	node := t.root
	for _, r := range strings.ToUpper(s) {
		child, ok := node.children[r]
		if !ok {
			return nil
		}
		node = child
	}
	return node
}
