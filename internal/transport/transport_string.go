// Code generated by "stringer -type=RequestState -output=transport_string.go"; DO NOT EDIT.

package transport

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Pending-1]
	_ = x[HeadersCompleted-2]
	_ = x[Error-3]
}

const _RequestState_name = "PendingHeadersCompletedError"

var _RequestState_index = [...]uint8{0, 7, 23, 28}

func (i RequestState) String() string {
	i -= 1
	if i >= RequestState(len(_RequestState_index)-1) {
		return "RequestState(" + strconv.FormatInt(int64(i+1), 10) + ")"
	}
	return _RequestState_name[_RequestState_index[i]:_RequestState_index[i+1]]
}
