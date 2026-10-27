//go:build windows && (amd64 || arm64)

package platform

import "unsafe"

// winMSG matches Win32 MSG on 64-bit: 48 bytes.
// HWND, UINT+pad, WPARAM, LPARAM, DWORD time, POINT, tail pad.
type winMSG struct {
	Hwnd    uintptr
	Message uint32
	_       uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
	_       uint32
}

var _ = ([1]byte{})[unsafe.Sizeof(winMSG{})-48]
