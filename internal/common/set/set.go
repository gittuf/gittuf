// Copyright The gittuf Authors
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
	if set == nil {
		return
	}

	for item := range set.contents {
		s.Add(item)
	}
}

func (s *Set[T]) Has(item T) bool {
	return s.contents[item]
}

func (s *Set[T]) Len() int {
	return len(s.contents)
}

func (s *Set[T]) Intersection(set *Set[T]) *Set[T] {
	intersection := NewSet[T]()

	rangeOver := s
	other := set
	if set.Len() < s.Len() {
		rangeOver = set
		other = s
	}

	for item := range rangeOver.contents {
		if other.Has(item) {
			intersection.Add(item)
		}
	}

	return intersection
}

func (s *Set[T]) Minus(set *Set[T]) *Set[T] {
	minus := NewSet[T]()

	for item := range s.contents {
		if !set.Has(item) {
			minus.Add(item)
		}
	}

	return minus
}

func (s *Set[T]) Equal(set *Set[T]) bool {
	if s.Len() != set.Len() {
		return false
	}

	for item := range s.contents {
		if !set.Has(item) {
			return false
		}
	}

	return true
}
