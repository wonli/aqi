package utils

import (
	"regexp"
	"strings"

	"github.com/mozillazg/go-pinyin"
)

// SensitiveTrie 敏感词前缀树
type SensitiveTrie struct {
	replaceChar rune // 敏感词替换的字符
	root        *TrieNode
}

// NewSensitiveTrie 构造敏感词前缀树实例
func NewSensitiveTrie() *SensitiveTrie {
	return &SensitiveTrie{
		replaceChar: '*',
		root:        &TrieNode{End: false},
	}
}

// AddWords 批量添加敏感词
func (s *SensitiveTrie) AddWords(sensitiveWords ...string) {
	a := pinyin.NewArgs()
	for _, sensitiveWord := range sensitiveWords {
		chnReg := regexp.MustCompile("[\u4e00-\u9fa5]")
		if chnReg.Match([]byte(sensitiveWord)) {
			// 只有中文才转
			lazyPy := pinyin.LazyPinyin(sensitiveWord, a)
			if lazyPy != nil {
				sFirstWords := ""
				for _, p := range lazyPy {
					sFirstWords += p[0:1]
				}

				s.addWord(sFirstWords)
				s.addWord(strings.Join(lazyPy, ""))
			}
		}

		s.addWord(sensitiveWord)
	}
}

// Match 查找替换发现的敏感词
func (s *SensitiveTrie) Match(text string) (sensitiveWords []string, replaceText string) {
	if s.root == nil {
		return nil, text
	}

	// 过滤特殊字符
	filteredText := s.filterSpecialChar(text)
	sensitiveMap := make(map[string]*struct{}) // 利用map把相同的敏感词去重
	textChars := []rune(filteredText)
	textCharsCopy := make([]rune, len(textChars))
	copy(textCharsCopy, textChars)
	for i, textLen := 0, len(textChars); i < textLen; i++ {
		trieNode := s.root.findChild(textChars[i])
		if trieNode == nil {
			continue
		}

		// 匹配到了敏感词的前缀，从后一个位置继续
		j := i + 1
		for ; j < textLen && trieNode != nil; j++ {
			if trieNode.End {
				// 完整匹配到了敏感词
				if _, ok := sensitiveMap[trieNode.Data]; !ok {
					sensitiveWords = append(sensitiveWords, trieNode.Data)
				}
				sensitiveMap[trieNode.Data] = nil

				// 将匹配的文本的敏感词替换成 *
				s.replaceRune(textCharsCopy, i, j)
			}
			trieNode = trieNode.findChild(textChars[j])
		}

		// 文本尾部命中敏感词情况
		if j == textLen && trieNode != nil && trieNode.End {
			if _, ok := sensitiveMap[trieNode.Data]; !ok {
				sensitiveWords = append(sensitiveWords, trieNode.Data)
			}
			sensitiveMap[trieNode.Data] = nil
			s.replaceRune(textCharsCopy, i, textLen)
		}
	}

	if len(sensitiveWords) > 0 {
		// 有敏感词
		replaceText = string(textCharsCopy)
	} else {
		// 没有则返回原来的文本
		replaceText = text
	}

	return sensitiveWords, replaceText
}

// AddWord 添加敏感词
func (s *SensitiveTrie) addWord(sensitiveWord string) {
	// 添加前先过滤一遍
	sensitiveWord = s.filterSpecialChar(sensitiveWord)

	// 将敏感词转换成utf-8编码后的rune类型(int32)
	tireNode := s.root
	sensitiveChars := []rune(sensitiveWord)
	for _, charInt := range sensitiveChars {
		// 添加敏感词到前缀树中
		tireNode = tireNode.addChild(charInt)
	}

	tireNode.End = true
	tireNode.Data = sensitiveWord
}

// replaceRune 字符替换
func (s *SensitiveTrie) replaceRune(chars []rune, begin int, end int) {
	for i := begin; i < end; i++ {
		chars[i] = s.replaceChar
	}
}

// filterSpecialChar 过滤特殊字符
func (s *SensitiveTrie) filterSpecialChar(text string) string {
	text = strings.ToLower(text)
	text = strings.Replace(text, " ", "", -1) // 去除空格
	return text
}

// TrieNode 敏感词前缀树节点
type TrieNode struct {
	childMap map[rune]*TrieNode // 本节点下的所有子节点
	Data     string             // 在最后一个节点保存完整的一个内容
	End      bool               // 标识是否最后一个节点
}

// addChild 前缀树添加字节点
func (n *TrieNode) addChild(c rune) *TrieNode {

	if n.childMap == nil {
		n.childMap = make(map[rune]*TrieNode)
	}

	if trieNode, ok := n.childMap[c]; ok {
		// 存在不添加了
		return trieNode
	} else {
		// 不存在
		n.childMap[c] = &TrieNode{
			childMap: nil,
			End:      false,
		}

		return n.childMap[c]
	}
}

// findChild 前缀树查找字节点
func (n *TrieNode) findChild(c rune) *TrieNode {
	if n.childMap == nil {
		return nil
	}

	if trieNode, ok := n.childMap[c]; ok {
		return trieNode
	}

	return nil
}
