//go:build linux && (amd64 || arm64)

package platform

import "unsafe"

// input_event is 24 bytes when timeval is two 64-bit fields.
var (
	_ = ([1]byte{})[unsafe.Sizeof(inputEvent{})-24]
	_ = ([1]byte{})[24-unsafe.Sizeof(inputEvent{})]
)
