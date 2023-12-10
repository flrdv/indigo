package json

import (
	"errors"
	"github.com/indigo-web/indigo/internal/flect"
	"github.com/indigo-web/utils/buffer"
	"github.com/indigo-web/utils/uf"
	"github.com/romshark/jscan/v2"
	"unsafe"
)

const (
	BufferSpaceMin = 1024
	BufferSpaceMax = 127 * 1024
)

var ErrNoSpace = errors.New("no space for values")

type JSON[T any] struct {
	model  flect.Model[T]
	buffer *buffer.Buffer[byte]
}

func NewJSON[T any]() *JSON[T] {
	return &JSON[T]{
		model:  flect.NewModel[T](),
		buffer: buffer.NewBuffer[byte](BufferSpaceMin, BufferSpaceMax),
	}
}

func (j *JSON[T]) Parse(input string) (result T, err error) {
	jsonErr := jscan.Scan(input, func(i *jscan.Iterator[string]) (exit bool) {
		key := i.Key()
		if len(i.Key()) == 0 || i.ValueType() != jscan.ValueTypeString {
			return false
		}

		field, found := j.model.Field(key[1 : len(key)-1])
		if !found {
			return false
		}

		if !j.buffer.Append(uf.S2B(i.Value())...) {
			err = ErrNoSpace
			return true
		}

		value := uf.B2S(j.buffer.Finish())
		value = value[1 : len(value)-1]
		result = field.WriteUFP(result, unsafe.Pointer(&value))

		return false
	})

	if jsonErr.IsErr() {
		err = jsonErr
	}

	return result, err
}

func (j *JSON[T]) Reset() {
	j.buffer.Clear()
}
