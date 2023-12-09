package json

import (
	"github.com/romshark/jscan/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type myJSONModel struct {
	Something string
	Nothing   string
	Boah      string
}

func TestJSON(t *testing.T) {
	j := `
	{
		"Boah": "any text here",
		"Something": "some text inside of the string",
		"Nothing": "Hello, world!",
		123: "This must never appear"
	}
	`
	parser := NewJSON[myJSONModel]()
	model, err := parser.Parse(j)
	require.NoError(t, err)
	assert.Equal(t, "some text inside of the string", model.Something)
	assert.Equal(t, "Hello, world!", model.Nothing)
	assert.Equal(t, "any text here", model.Boah)
}

func BenchmarkJSON(b *testing.B) {
	j := `
	{
		"Boah": 857,
		"Something": "okay, let it be",
		"Nothing": "Hello, world!",
		123: "This must never appear"
	}
	`

	parser := NewJSON[myJSONModel]()
	m := myJSONModel{}

	b.Run("my parser", func(b *testing.B) {
		b.SetBytes(int64(len(j)))
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			m, _ = parser.Parse(j)
		}
	})

	b.Run("without my parser", func(b *testing.B) {
		b.SetBytes(int64(len(j)))
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = jscan.Scan(j, func(i *jscan.Iterator[string]) (err bool) {
				return false
			})
		}
	})

	keepalive(m)
}

func keepalive(myJSONModel) {}
