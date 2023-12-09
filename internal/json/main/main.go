package main

import (
	"fmt"
	"github.com/indigo-web/indigo/internal/json"
)

type myJSONModel struct {
	Something string
	Nothing   string
	Boah      string
}

func (m myJSONModel) String() string {
	return fmt.Sprintf(
		"myJSONModel{Something: %s, Nothing: %s, Boah: %s}", m.Something, m.Nothing, m.Boah,
	)
}

func main() {
	j := `
	{
		"Boah": 857,
		"Something": "okay, let it be",
		"Nothing": "Hello, world!",
		123: "This must never appear"
	}
	`

	parser := json.NewJSON[myJSONModel]()
	result, err := parser.Parse(j)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(result.String())
}
