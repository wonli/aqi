package utils

import (
	"log"
	"testing"
)

func TestSensitive(t *testing.T) {
	sensitiveWords := []string{
		"牛大大", "撒比",
	}

	trie := NewSensitiveTrie()
	trie.AddWords(sensitiveWords...)

	content := "今天，牛大大挑战sb灰大大"
	matchSensitiveWords, replaceText := trie.Match(content)

	log.Printf("%v", trie)
	log.Println(matchSensitiveWords, replaceText)
}
