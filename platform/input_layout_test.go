package platform

import (
	"testing"
	"unsafe"
)

func TestWindowsInputLayout(t *testing.T) {
	ptr := unsafe.Sizeof(uintptr(0))
	var wantSize, wantMI, wantExtra uintptr
	switch ptr {
	case 8:
		wantSize, wantMI, wantExtra = 40, 8, 32
	case 4:
		wantSize, wantMI, wantExtra = 28, 4, 24
	default:
		t.Fatalf("unexpected pointer size %d", ptr)
	}
	if got := unsafe.Sizeof(input{}); got != wantSize {
		t.Fatalf("INPUT size %d, want %d", got, wantSize)
	}
	if got := unsafe.Offsetof(input{}.mi); got != wantMI {
		t.Fatalf("MOUSEINPUT offset %d, want %d", got, wantMI)
	}
	extra := unsafe.Offsetof(input{}.mi) + unsafe.Offsetof(mouseInput{}.dwExtraInfo)
	if extra != wantExtra {
		t.Fatalf("dwExtraInfo offset %d, want %d", extra, wantExtra)
	}
}
