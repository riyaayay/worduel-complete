package trie

import "testing"

func TestInsertAndIsValidWord(t *testing.T) {
	tr := New()
	tr.Load([]string{"cat", "car", "card", "care"})

	cases := map[string]bool{
		"cat":  true,
		"CAT":  true,
		"car":  true,
		"card": true,
		"care": true,
		"ca":   false, // prefix only, not a full word
		"dog":  false,
	}
	for word, want := range cases {
		if got := tr.IsValidWord(word); got != want {
			t.Errorf("IsValidWord(%q) = %v, want %v", word, got, want)
		}
	}
}

func TestWordsWithPrefix(t *testing.T) {
	tr := New()
	tr.Load([]string{"cat", "car", "card", "care", "dog"})

	got := tr.WordsWithPrefix("car")
	want := map[string]bool{"CAR": true, "CARD": true, "CARE": true}
	if len(got) != len(want) {
		t.Fatalf("WordsWithPrefix(car) = %v, want 3 words", got)
	}
	for _, w := range got {
		if !want[w] {
			t.Errorf("unexpected word %q in results", w)
		}
	}
}

func TestSizeCountsUniqueWordsOnly(t *testing.T) {
	tr := New()
	tr.Load([]string{"cat", "cat", "dog"})
	if tr.Size() != 2 {
		t.Errorf("Size() = %d, want 2", tr.Size())
	}
}

func TestLoadDefaultDictionary(t *testing.T) {
	tr := LoadDefaultDictionary()
	if tr.Size() < 1000 {
		t.Fatalf("expected a substantial embedded dictionary, got %d words", tr.Size())
	}
	if !tr.IsValidWord("word") {
		t.Errorf("expected 'word' to be a valid dictionary word")
	}
	if tr.IsValidWord("zzzxxxqqq") {
		t.Errorf("expected nonsense string to be invalid")
	}
}
