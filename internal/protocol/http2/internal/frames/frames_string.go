// Code generated by "stringer -type=Frame -output=frames_string.go"; DO NOT EDIT.

package frames

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Data-0]
	_ = x[Headers-1]
	_ = x[Priority-2]
	_ = x[RstStream-3]
	_ = x[Settings-4]
	_ = x[PushPromise-5]
	_ = x[Ping-6]
	_ = x[GoAway-7]
	_ = x[WindowUpdate-8]
	_ = x[Continuation-9]
	_ = x[Origin-12]
}

const (
	_Frame_name_0 = "DataHeadersPriorityRstStreamSettingsPushPromisePingGoAwayWindowUpdateContinuation"
	_Frame_name_1 = "Origin"
)

var (
	_Frame_index_0 = [...]uint8{0, 4, 11, 19, 28, 36, 47, 51, 57, 69, 81}
)

func (i Frame) String() string {
	switch {
	case i <= 9:
		return _Frame_name_0[_Frame_index_0[i]:_Frame_index_0[i+1]]
	case i == 12:
		return _Frame_name_1
	default:
		return "Frame(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}