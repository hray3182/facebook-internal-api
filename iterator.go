package fbia

import "iter"

// Collect drains an iter.Seq2[T, error] into a slice, stopping on the first error.
func Collect[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var items []T
	for item, err := range seq {
		if err != nil {
			return items, err
		}
		items = append(items, item)
	}
	return items, nil
}
