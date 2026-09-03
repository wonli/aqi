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
			node.Count++
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

// Del removes a word while preserving any shared prefix and descendants.
// Deleting a word that does not exist is a no-op.
func (t *Trie) Del(str string) {
	runes := []rune(str)
	if len(runes) == 0 {
		return
	}

	nodes := make([]*Node, len(runes)+1)
	nodes[0] = t.Root

	node := t.Root
	for i, val := range runes {
		child, ok := node.Child[val]
		if !ok {
			return
		}
		node = child
		nodes[i+1] = node
	}

	if !node.IsWord {
		return
	}

	node.IsWord = false
	node.Payload = nil

	for i := len(runes) - 1; i >= 0; i-- {
		child := nodes[i+1]
		if child.IsWord || len(child.Child) > 0 {
			break
		}

		parent := nodes[i]
		delete(parent.Child, runes[i])
		parent.Count = len(parent.Child)
	}
}
