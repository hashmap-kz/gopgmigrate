package ds

import (
	"errors"
)

// Stack represents a generic stack using a slice
type Stack[T any] struct {
	items []T
}

// NewStack initializes a new stack and pushes all elements from the given slice in reverse order.
func NewStack[T any](elements []T) *Stack[T] {
	stack := &Stack[T]{}

	// Push elements in reverse order
	for i := len(elements) - 1; i >= 0; i-- {
		stack.Push(elements[i])
	}

	return stack
}

// Push adds an item to the top of the stack
func (s *Stack[T]) Push(item T) {
	s.items = append(s.items, item)
}

// Pop removes and returns the top item from the stack
func (s *Stack[T]) Pop() (T, error) {
	if len(s.items) == 0 {
		var zeroVal T
		return zeroVal, errors.New("stack is empty")
	}

	// Get the last item
	lastIndex := len(s.items) - 1
	item := s.items[lastIndex]

	// Remove the last item
	s.items = s.items[:lastIndex]

	return item, nil
}

// Peek returns the top element without removing it
func (s *Stack[T]) Peek() (T, error) {
	if len(s.items) == 0 {
		var zeroVal T
		return zeroVal, errors.New("stack is empty")
	}
	return s.items[len(s.items)-1], nil
}

// IsEmpty checks if the stack is empty
func (s *Stack[T]) IsEmpty() bool {
	return len(s.items) == 0
}

// Size returns the number of elements in the stack
func (s *Stack[T]) Size() int {
	return len(s.items)
}
