package huffman

// TODO: as the table is hard-coded, we can use something more efficient instead of the binary tree

var Tree = newPtrTree()

type PtrNode struct {
	Char        byte
	IsLeaf      bool
	Left, Right *PtrNode
}

func newPtrTree() *PtrNode {
	root := new(PtrNode)

	for char, symbol := range Table {
		node := root

		for i := symbol.Bits; i > 0; i-- {
			if symbol.Code&(uint32(1)<<(i-1)) != 0 {
				if node.Right == nil {
					node.Right = new(PtrNode)
				}

				node = node.Right
			} else {
				if node.Left == nil {
					node.Left = new(PtrNode)
				}

				node = node.Left
			}
		}

		node.Char = byte(char)
		node.IsLeaf = true
	}

	return root
}

func Decompress(data, out []byte) ([]byte, bool) {
	node := Tree
	allSet := true

	for _, b := range data {
		for i := uint8(0); i < 8; i++ {
			if b&(0x80>>i) != 0 {
				node = node.Right
			} else {
				node = node.Left
				allSet = false
			}

			if node == nil {
				return out[:0], false
			}

			if node.IsLeaf {
				out = append(out, node.Char)
				node = Tree
				allSet = true
			}
		}
	}

	return out, allSet
}
