package datastructures

type Heap[T any] struct {
	arr     []T
	compare func(a, b any) bool
}

func NewHeap[T any](compare func(a, b any) bool) *Heap[T] {
	return &Heap[T]{compare: compare}
}

func (h *Heap[T]) Push(key T) {
	h.arr = append(h.arr, key)
	h.upHeapify(len(h.arr) - 1)
}

func (h *Heap[T]) Pop() T {
	root := h.arr[0]
	h.arr[0] = h.arr[len(h.arr)-1]
	h.arr = h.arr[:len(h.arr)-1]
	h.downHeapify(0)
	return root
}

func (h *Heap[T]) Len() int {
	return len(h.arr)
}

func (h *Heap[T]) upHeapify(i int) {
	parent := (i - 1) / 2
	if i != 0 && h.compare(h.arr[i], h.arr[parent]) {
		h.arr[i], h.arr[parent] = h.arr[parent], h.arr[i]
		h.upHeapify(parent)
	}
}

func (h *Heap[T]) downHeapify(i int) {
	left := 2*i + 1
	right := 2*i + 2
	smallest := i
	if left < len(h.arr) && h.compare(h.arr[left], h.arr[i]) {
		smallest = left
	}
	if right < len(h.arr) && h.compare(h.arr[right], h.arr[smallest]) {
		smallest = right
	}
	if smallest != i {
		h.arr[i], h.arr[smallest] = h.arr[smallest], h.arr[i]
		h.downHeapify(smallest)
	}
}
