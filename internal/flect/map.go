package flect

type attrsMap[T any] struct {
	buckets []attrsMapBucket[T]
}

func (a *attrsMap[T]) Lookup(key string) (field Field[T], found bool) {
	if len(key) > len(a.buckets)-1 {
		return field, false
	}

	for _, entry := range a.buckets[len(key)] {
		if entry.Key == key {
			return entry.Field, true
		}
	}

	return field, false
}

func (a *attrsMap[T]) Insert(key string, field Field[T]) {
	if len(a.buckets) < len(key) {
		a.grow(len(key))
	}

	a.buckets[len(key)] = append(a.buckets[len(key)], attrsMapEntry[T]{
		Key:   key,
		Field: field,
	})
}

func (a *attrsMap[T]) grow(n int) {
	newBuckets := make([]attrsMapBucket[T], n+1)
	copy(newBuckets, a.buckets)
	a.buckets = newBuckets
}

type attrsMapBucket[T any] []attrsMapEntry[T]

type attrsMapEntry[T any] struct {
	Key   string
	Field Field[T]
}
