//go:build linux

package platform

import (
	"encoding/binary"
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

func TestUinputEventEncoding(t *testing.T) {
	ev := inputEvent{
		Time:  unix.Timeval{Sec: 0x11223344, Usec: 0x55667788},
		Type:  0x0102,
		Code:  btnLeft,
		Value: 0x05060708,
	}
	b := marshalInputEvent(ev)
	if len(b) != int(unsafe.Sizeof(ev)) {
		t.Fatalf("len %d sizeof %d", len(b), unsafe.Sizeof(ev))
	}
	back := *(*inputEvent)(unsafe.Pointer(&b[0]))
	if back.Time != ev.Time || back.Type != ev.Type || back.Code != ev.Code || back.Value != ev.Value {
		t.Fatalf("round trip %+v", back)
	}
	if runtime.GOARCH == "amd64" {
		if len(b) != 24 {
			t.Fatalf("size %d, want 24", len(b))
		}
		if binary.LittleEndian.Uint64(b[0:8]) != uint64(ev.Time.Sec) {
			t.Fatalf("sec %#x", binary.LittleEndian.Uint64(b[0:8]))
		}
		if binary.LittleEndian.Uint64(b[8:16]) != uint64(ev.Time.Usec) {
			t.Fatalf("usec %#x", binary.LittleEndian.Uint64(b[8:16]))
		}
		if binary.LittleEndian.Uint16(b[16:18]) != ev.Type {
			t.Fatalf("type %#x", binary.LittleEndian.Uint16(b[16:18]))
		}
		if binary.LittleEndian.Uint16(b[18:20]) != ev.Code {
			t.Fatalf("code %#x", binary.LittleEndian.Uint16(b[18:20]))
		}
		if int32(binary.LittleEndian.Uint32(b[20:24])) != ev.Value {
			t.Fatalf("value %#x", binary.LittleEndian.Uint32(b[20:24]))
		}
	}
}

func TestParseScreenSize(t *testing.T) {
	w, h, err := parseScreenSize("1280x800")
	if err != nil || w != 1280 || h != 800 {
		t.Fatalf("%d %d %v", w, h, err)
	}
	w, h, err = parseScreenSize(" 1920X1080 ")
	if err != nil || w != 1920 || h != 1080 {
		t.Fatalf("%d %d %v", w, h, err)
	}
	if _, _, err := parseScreenSize("0x10"); err == nil {
		t.Fatal("expected error")
	}
	if _, _, err := parseScreenSize("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewUinputRejectsNonPositive(t *testing.T) {
	// Must fail before opening /dev/uinput.
	if _, err := NewUinput(0, 800); err == nil {
		t.Fatal("expected error")
	}
	if _, err := NewUinput(100, -1); err == nil {
		t.Fatal("expected error")
	}
}

func TestUinputCursorUnsupported(t *testing.T) {
	var u uinputClicker
	x, y, err := u.CursorPos()
	if x != 0 || y != 0 || err != ErrUinputCursor {
		t.Fatalf("%d %d %v", x, y, err)
	}
}
