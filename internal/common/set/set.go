// SPDX-License-Identifier: Apache-2.0

package set

type Set[T comparable] struct {
	contents map[T]bool
}

func NewSet[T comparable]() *Set[T] {
	return &Set[T]{contents: map[T]bool{}}
}

func (s *Set[T]) Contents() []T {
	items := []T{}
	for item := range s.contents {
		items = append(items, item)
	}
	return items
}

func (s *Set[T]) Add(item T) {
	s.contents[item] = true
}

func (s *Set[T]) Remove(item T) {
	delete(s.contents, item)
}

func (s *Set[T]) Extend(set *Set[T]) {
	items := set.Contents()
	for _, item := range items {
		s.Add(item)
	}
}

func (s *Set[T]) Has(item T) bool {
	return s.contents[item]
}

func (s *Set[T]) Len() int {
	return len(s.contents)
}
