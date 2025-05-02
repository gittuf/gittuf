// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package set

import (
	"cmp"
	"encoding/json"
	"slices"
)

// Set implements a generic set data structure for use in gittuf metadata and
// workflows.
type Set[T cmp.Ordered] struct {
	contents map[T]struct{}
}

// NewSet creates a new instance of a set for the specified type that fulfils
// the cmp.Ordered constraint.
func NewSet[T cmp.Ordered]() *Set[T] {
	return &Set[T]{contents: map[T]struct{}{}}
}

// NewSetFromItems creates a new instance of a set and populates it with the
// items provided.
func NewSetFromItems[T cmp.Ordered](items ...T) *Set[T] {
	set := NewSet[T]()
	for _, item := range items {
		set.Add(item)
	}

	return set
}

// MarshalJSON is used to serialize the instance of the set into JSON.
func (s *Set[T]) MarshalJSON() ([]byte, error) {
	contents := s.Contents()
	slices.Sort(contents)

	return json.Marshal(contents)
}

// UnmarshalJSON is used to load a set from the JSON representation.
func (s *Set[T]) UnmarshalJSON(jsonBytes []byte) error {
	items := []T{}
	if err := json.Unmarshal(jsonBytes, &items); err != nil {
		return err
	}

	s.contents = map[T]struct{}{}
	for _, item := range items {
		s.Add(item)
	}

	return nil
}

// Contents returns the objects present in the set.
func (s *Set[T]) Contents() []T {
	if s.contents == nil {
		return nil
	}

	items := []T{}
	for item := range s.contents {
		items = append(items, item)
	}
	return items
}

// Add inserts an item into the set.
func (s *Set[T]) Add(item T) {
	s.contents[item] = struct{}{}
}

// Remove deletes the item from the set.
func (s *Set[T]) Remove(item T) {
	delete(s.contents, item)
}

// Extend adds all of the items in the passed set, resulting in a union
// operation.
func (s *Set[T]) Extend(set *Set[T]) {
	if set == nil {
		return
	}

	for item := range set.contents {
		s.Add(item)
	}
}

// Has returns true if the set has the corresponding item.
func (s *Set[T]) Has(item T) bool {
	_, ok := s.contents[item]
	return ok
}

// Len returns the number of objects in the set.
func (s *Set[T]) Len() int {
	return len(s.contents)
}

// Intersection returns a new set consisting of the items present in both sets.
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

// Minus returns a new set consisting of the items present only in the current
// set.
func (s *Set[T]) Minus(set *Set[T]) *Set[T] {
	minus := NewSet[T]()

	for item := range s.contents {
		if !set.Has(item) {
			minus.Add(item)
		}
	}

	return minus
}

// Equal returns true if both sets have the same items.
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
