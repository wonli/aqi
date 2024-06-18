package ws

import "sync"

type (
	Stack struct {
		top    *node
		length int
		lock   *sync.RWMutex
	}

	node struct {
		value any
		prev  *node
	}
)

func NewStack() *Stack {
	return &Stack{
		top:    nil,
		length: 0,
		lock:   &sync.RWMutex{},
	}
}

func (s *Stack) Len() int {
	return s.length
}

func (s *Stack) Pop() any {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.length == 0 {
		return nil
	}

	n := s.top
	s.top = n.prev
	s.length = s.length - 1
	return n.value
}

func (s *Stack) Push(value any) {
	s.lock.Lock()
	defer s.lock.Unlock()

	n := &node{value: value, prev: s.top}
	s.top = n
	s.length = s.length + 1
}
