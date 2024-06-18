package tree

type Node struct {
	Val     rune           // The value of the node
	Depth   int            // The depth of the node in the tree
	Count   int            // Counts the number of branches
	Payload interface{}    // The payload associated with the node
	Child   map[rune]*Node // Children of the node, mapped by rune
	IsWord  bool           // Flag indicating if this node marks the end of a complete string
}

// NewNode new node
func NewNode() *Node {
	return &Node{Child: make(map[rune]*Node)}
}

type Trie struct {
	Root *Node
}

func NewTrie() *Trie {
	return &Trie{Root: NewNode()}
}

// Insert node
func (t *Trie) Insert(str string, p any) {
	if len(str) == 0 {
		return
	}

	bt := []rune(str)
	node := t.Root
	for _, val := range bt {
		child, ok := node.Child[val]
		if !ok {
			child = NewNode()
			child.Val = val
			node.Child[val] = child
			node.Count += 1
			child.Depth = node.Depth + 1
		}
		node = child
	}

	node.Payload = p
	node.IsWord = true
}

func (t *Trie) Find(str string) (bool, interface{}) {
	bt := []rune(str)
	node := t.Root

	for _, val := range bt {
		child, ok := node.Child[val]
		if !ok {
			return false, nil
		}
		node = child
	}
	return node.IsWord, node.Payload
}

// FindAll finds all strings that start with the given prefix and returns their payloads.
func (t *Trie) FindAll(prefix string) []any {
	bt := []rune(prefix)
	node := t.Root

	for _, val := range bt {
		child, ok := node.Child[val]
		if !ok {
			return nil
		}

		node = child
	}

	return t.collect(node)
}

// collect Recursively collects all strings' payloads in the subtree rooted at the given node.
func (t *Trie) collect(node *Node) (payloads []any) {
	if node.IsWord {
		payloads = append(payloads, node.Payload)
	}

	for _, childNode := range node.Child {
		payloads = append(payloads, t.collect(childNode)...)
	}

	return payloads
}

// Del deletion of a node has the following cases:
//  1. Prefix deletion: Check if Count is greater than 0, then set IsWord to false.
//  3. String deletion:
//     a. If there is no branching, delete the entire string.
//     b. If there is branching, only delete the part that is not a common prefix.
func (t *Trie) Del(str string) {
	bt := []rune(str)
	if len(str) == 0 {
		return
	}

	node := t.Root
	var lastBranch *Node
	var delVal rune

	for index, val := range bt {
		child, ok := node.Child[val]
		if ok {
			if child.Count > 1 {
				lastBranch = child
				delVal = bt[index+1]
			}
		}
		node = child
	}

	if node.Count > 0 {
		// del prefix
		node.IsWord = false
	} else {
		if lastBranch == nil {
			// del charset
			lastBranch = t.Root
			delVal = bt[0]
		}
		delete(lastBranch.Child, delVal)
		lastBranch.Count -= 1
	}
}
