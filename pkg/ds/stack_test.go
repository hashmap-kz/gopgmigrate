package ds

import (
	"testing"
)

// TestNewStack checks if NewStack initializes a stack with elements in reverse order
func TestNewStack(t *testing.T) {
	data := []int{1, 2, 3, 4, 5}
	stack := NewStack(data)

	if stack.Size() != len(data) {
		t.Errorf("expected stack size %d, got %d", len(data), stack.Size())
	}

	// Ensure elements are pushed in reverse order
	for i := 0; i < len(data); i++ {
		item, err := stack.Pop()
		if err != nil {
			t.Errorf("unexpected error on Pop: %v", err)
		}
		if item != data[i] {
			t.Errorf("expected %d, got %d", data[i], item)
		}
	}
}

// TestPush checks if Push correctly adds an element to the stack
func TestPush(t *testing.T) {
	stack := &Stack[int]{}
	stack.Push(10)

	if stack.Size() != 1 {
		t.Errorf("expected stack size 1, got %d", stack.Size())
	}

	item, err := stack.Peek()
	if err != nil {
		t.Errorf("unexpected error on Peek: %v", err)
	}
	if item != 10 {
		t.Errorf("expected 10, got %d", item)
	}
}

// TestPop checks if Pop correctly removes and returns the top element
func TestPop(t *testing.T) {
	stack := &Stack[int]{}
	stack.Push(42)

	item, err := stack.Pop()
	if err != nil {
		t.Errorf("unexpected error on Pop: %v", err)
	}
	if item != 42 {
		t.Errorf("expected 42, got %d", item)
	}

	// Ensure stack is empty after pop
	if !stack.IsEmpty() {
		t.Errorf("expected stack to be empty")
	}

	// Test popping from an empty stack
	_, err = stack.Pop()
	if err == nil {
		t.Errorf("expected an error when popping from an empty stack")
	}
}

// TestPeek checks if Peek correctly returns the top element without removing it
func TestPeek(t *testing.T) {
	stack := &Stack[int]{}
	stack.Push(99)

	item, err := stack.Peek()
	if err != nil {
		t.Errorf("unexpected error on Peek: %v", err)
	}
	if item != 99 {
		t.Errorf("expected 99, got %d", item)
	}

	// Ensure size remains the same after Peek
	if stack.Size() != 1 {
		t.Errorf("expected stack size 1 after Peek, got %d", stack.Size())
	}
}

// TestIsEmpty checks if IsEmpty correctly identifies an empty stack
func TestIsEmpty(t *testing.T) {
	stack := &Stack[int]{}
	if !stack.IsEmpty() {
		t.Errorf("expected stack to be empty")
	}

	stack.Push(5)
	if stack.IsEmpty() {
		t.Errorf("expected stack to be non-empty")
	}
}

// TestSize checks if Size correctly returns the number of elements
func TestSize(t *testing.T) {
	stack := &Stack[int]{}

	if stack.Size() != 0 {
		t.Errorf("expected stack size 0, got %d", stack.Size())
	}

	stack.Push(1)
	stack.Push(2)
	stack.Push(3)

	if stack.Size() != 3 {
		t.Errorf("expected stack size 3, got %d", stack.Size())
	}
}
