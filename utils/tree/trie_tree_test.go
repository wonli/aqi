package tree

import (
	"sort"
	"testing"
)

func TestTrieInsertFindAndFindAll(t *testing.T) {
	trie := NewTrie()
	trie.Insert("app", "app")
	trie.Insert("apple", "apple")
	trie.Insert("apply", "apply")
	trie.Insert("中文", "zh")

	if ok, payload := trie.Find("app"); !ok || payload != "app" {
		t.Fatalf("Find(app) = (%v, %#v), want (true, app)", ok, payload)
	}
	if ok, _ := trie.Find("ap"); ok {
		t.Fatal("prefix without inserted word should not match")
	}
	if ok, payload := trie.Find("中文"); !ok || payload != "zh" {
		t.Fatalf("Find(中文) = (%v, %#v), want (true, zh)", ok, payload)
	}

	got := trie.FindAll("app")
	values := make([]string, 0, len(got))
	for _, value := range got {
		values = append(values, value.(string))
	}
	sort.Strings(values)
	want := []string{"app", "apple", "apply"}
	if len(values) != len(want) {
		t.Fatalf("FindAll(app) = %v, want %v", values, want)
	}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("FindAll(app) = %v, want %v", values, want)
		}
	}
}

func TestTrieDelete(t *testing.T) {
	t.Run("deleting word keeps longer words", func(t *testing.T) {
		trie := NewTrie()
		trie.Insert("app", "app")
		trie.Insert("apple", "apple")

		trie.Del("app")

		if ok, _ := trie.Find("app"); ok {
			t.Fatal("deleted word app still exists")
		}
		if ok, payload := trie.Find("apple"); !ok || payload != "apple" {
			t.Fatal("deleting prefix word removed longer word")
		}
	})

	t.Run("deleting leaf keeps sibling", func(t *testing.T) {
		trie := NewTrie()
		trie.Insert("apple", "apple")
		trie.Insert("apply", "apply")

		trie.Del("apple")

		if ok, _ := trie.Find("apple"); ok {
			t.Fatal("deleted leaf apple still exists")
		}
		if ok, payload := trie.Find("apply"); !ok || payload != "apply" {
			t.Fatal("deleting apple removed sibling apply")
		}
	})

	t.Run("deleting missing word is no-op", func(t *testing.T) {
		trie := NewTrie()
		trie.Insert("app", "app")

		trie.Del("missing")

		if ok, payload := trie.Find("app"); !ok || payload != "app" {
			t.Fatal("deleting missing word changed trie")
		}
	})
}
