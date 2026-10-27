//go:build windows && (amd64 || arm64)

package platform

import "unsafe"

// Compile-time check: Win32 INPUT is 40 bytes on 64-bit.
var _ = ([1]byte{})[unsafe.Sizeof(input{})-40]
