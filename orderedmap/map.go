package orderedmap

import (
	"constraints"
	"sort"
)

// OrderedMap is a map that can be iterated in the natural order of its keys.
//
// TODO implement json.Marshaler and json.Unmarshaler
type OrderedMap[K constraints.Ordered, V any] map[K]V

// Iterate iterates over the keys of the OrderedMap in natural sort order. If the iterator returns false, iteration is
// aborted (like the break keyword).
//
// Iterate is not safe for concurrent modification.
func (m OrderedMap[K, V]) Iterate(iterator func(key K, value V) bool) {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		if !iterator(k, m[k]) {
			break
		}
	}
}
