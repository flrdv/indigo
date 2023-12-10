package flect

import (
	"reflect"
	"unsafe"
)

type Model[T any] struct {
	attrs *attrsMap[T]
}

func NewModel[T any]() Model[T] {
	var zero [0]T
	typ := reflect.TypeOf(zero).Elem()
	if typ.Kind() != reflect.Struct {
		panic("not a struct")
	}

	model := Model[T]{
		attrs: new(attrsMap[T]),
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		model.attrs.Insert(field.Name, Field[T]{
			meta: fieldMeta{
				Size:   field.Type.Size(),
				Offset: field.Offset,
			},
		})
	}

	return model
}

func (m Model[T]) Field(key string) (Field[T], bool) {
	return m.attrs.Lookup(key)
}

type Attr struct {
	Key   string
	Value unsafe.Pointer
}

func Instantiate[T any](model Model[T], attrs ...Attr) T {
	var zero T

	for _, attr := range attrs {
		field, found := model.Field(attr.Key)
		if !found {
			continue
		}

		zero = field.WriteUFP(zero, attr.Value)
	}

	return zero
}

type Field[T any] struct {
	meta fieldMeta
}

func (f Field[T]) WriteUFP(into T, src unsafe.Pointer) T {
	dst := unsafe.Add(unsafe.Pointer(&into), f.meta.Offset)
	memcpy(dst, src, f.meta.Size)

	return into
}

func (f Field[T]) WriteUInt8(into T, num uint8) T {
	return f.WriteUFP(into, unsafe.Pointer(&num))
}

func (f Field[T]) WriteUInt16(into T, num uint16) T {
	return f.WriteUFP(into, unsafe.Pointer(&num))
}

func (f Field[T]) WriteUInt32(into T, num uint32) T {
	return f.WriteUFP(into, unsafe.Pointer(&num))
}

func (f Field[T]) WriteUInt64(into T, num uint64) T {
	return f.WriteUFP(into, unsafe.Pointer(&num))
}

func (f Field[T]) WriteInt8(into T, num int8) T {
	return f.WriteUFP(into, unsafe.Pointer(&num))
}

func (f Field[T]) WriteInt16(into T, num int16) T {
	return f.WriteUFP(into, unsafe.Pointer(&num))
}

func (f Field[T]) WriteInt32(into T, num int32) T {
	return f.WriteUFP(into, unsafe.Pointer(&num))
}

func (f Field[T]) WriteInt64(into T, num int64) T {
	return f.WriteUFP(into, unsafe.Pointer(&num))
}

func (f Field[T]) WriteString(into T, value string) T {
	return f.WriteUFP(into, unsafe.Pointer(&value))
}

func memcpy(dst, src unsafe.Pointer, size uintptr) {
	copy(unsafe.Slice((*byte)(dst), size), unsafe.Slice((*byte)(src), size))
}

type fieldMeta struct {
	Size, Offset uintptr
}
