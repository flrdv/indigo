package bit

type Map8 uint8

// Set sets the chosen bit to 1.
func (m *Map8) Set(pos uint8) {
	*m |= Map8(1) << pos
}

// Unset sets the chosen bit to 0.
func (m *Map8) Unset(pos uint8) {
	*m &^= Map8(1) << pos
}

// Is returns whether the chosen bit is set.
func (m *Map8) Is(pos uint8) bool {
	return *m&(Map8(1)<<pos) != 0
}

// Clear unsets all bits.
func (m *Map8) Clear() {
	*m = 0
}
