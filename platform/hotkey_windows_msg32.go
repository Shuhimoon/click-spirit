//go:build windows && 386

package platform

import "unsafe"

// winMSG matches Win32 MSG on 32-bit: 28 bytes.
type winMSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
}

var _ = ([1]byte{})[unsafe.Sizeof(winMSG{})-28]
