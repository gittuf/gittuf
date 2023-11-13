package datastructures

import "testing"

func TestHeap(t *testing.T) {
	type fields struct {
		arr     []int
		compare func(a, b interface{}) bool
	}
	tests := []struct {
		name           string
		fields         fields
		expectedPopped []int
	}{
		{
			name: "random",
			fields: fields{
				arr: []int{1, 4, 3, 2, 5},
				compare: func(a, b interface{}) bool {
					return a.(int) < b.(int)
				},
			},
			expectedPopped: []int{1, 2, 3, 4, 5},
		},
		{
			name: "empty",
			fields: fields{
				arr: []int{},
				compare: func(a, b interface{}) bool {
					return a.(int) < b.(int)
				},
			},
			expectedPopped: nil,
		},
		{
			name: "reverse",
			fields: fields{
				arr: []int{5, 4, 3, 2, 1},
				compare: func(a, b interface{}) bool {
					return a.(int) < b.(int)
				},
			},
			expectedPopped: []int{1, 2, 3, 4, 5},
		},
		{
			name: "same elements",
			fields: fields{
				arr: []int{2, 2, 2, 2, 2},
				compare: func(a, b interface{}) bool {
					return a.(int) < b.(int)
				},
			},
			expectedPopped: []int{2, 2, 2, 2, 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHeap[int](tt.fields.compare)
			for _, v := range tt.fields.arr {
				h.Push(v)
			}
			for i := 0; h.Len() > 0; i++ {
				if val := h.Pop(); val != tt.expectedPopped[i] {
					t.Fatalf("Mismatch at heap pop %v: expected = %v, got = %v", i, tt.expectedPopped[i], val)
				}
			}
		})
	}
}
