//go:build windows && 386

package platform

import "unsafe"

// Compile-time check: Win32 INPUT is 28 bytes on 32-bit.
var _ = ([1]byte{})[unsafe.Sizeof(input{})-28]
