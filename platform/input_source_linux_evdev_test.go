//go:build linux

package platform

import (
	"encoding/binary"
	"errors"
	"os"
	"strings"
	"testing"
	"unsafe"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/core/record"
)

func TestParseInputEventBytes(t *testing.T) {
	ev := inputEvent{Type: evRel, Code: relY, Value: -3}
	raw := marshalInputEvent(ev)
	raw = append(raw, 0xab)
	got, rest := parseInputEventBytes(raw)
	if len(got) != 1 || got[0].Type != evRel || got[0].Code != relY || got[0].Value != -3 {
		t.Fatalf("%+v", got)
	}
	if len(rest) != 1 || rest[0] != 0xab {
		t.Fatalf("rest %v", rest)
	}
	if _, rest := parseInputEventBytes(nil); len(rest) != 0 {
		t.Fatalf("rest %v", rest)
	}
}

func TestInputEventBytesMatch64Layout(t *testing.T) {
	if unsafe.Sizeof(inputEvent{}) != 24 {
		t.Skip("input_event is 24 bytes on amd64 and arm64")
	}
	b := make([]byte, 24)
	binary.LittleEndian.PutUint64(b[0:8], 3)
	binary.LittleEndian.PutUint64(b[8:16], 4)
	binary.LittleEndian.PutUint16(b[16:18], evKey)
	binary.LittleEndian.PutUint16(b[18:20], btnLeft)
	binary.LittleEndian.PutUint32(b[20:24], 1)
	got, rest := parseInputEventBytes(b)
	if len(rest) != 0 || len(got) != 1 {
		t.Fatalf("%+v rest %d", got, len(rest))
	}
	if got[0].Type != evKey || got[0].Code != btnLeft || got[0].Value != 1 {
		t.Fatalf("%+v", got[0])
	}
	if got[0].Time.Sec != 3 || got[0].Time.Usec != 4 {
		t.Fatalf("time %+v", got[0].Time)
	}
}

func TestEvdevPointerRelativeClampAndButtons(t *testing.T) {
	p := pointerTrack{w: 100, h: 50}
	dev := &evdevDev{}
	events := []inputEvent{
		{Type: evRel, Code: relX, Value: 10},
		{Type: evRel, Code: relY, Value: 4},
		{Type: evKey, Code: btnLeft, Value: 1},
		{Type: evKey, Code: btnLeft, Value: 0},
		{Type: evKey, Code: btnLeft, Value: 2},
		{Type: evRel, Code: relX, Value: -1000},
		{Type: evRel, Code: relY, Value: 1000},
		{Type: evKey, Code: btnRight, Value: 1},
		{Type: evKey, Code: btnMiddle, Value: 1},
		{Type: evRel, Code: 0x08, Value: 1}, // wheel, ignored
	}
	var got []record.Event
	for _, ev := range events {
		out, ok := applyEvdevPointer(&p, dev, ev)
		if ok {
			got = append(got, out)
		}
	}
	want := []record.Event{
		{Kind: record.KindMove, X: 10, Y: 0},
		{Kind: record.KindMove, X: 10, Y: 4},
		{Kind: record.KindDown, Button: core.ButtonLeft, X: 10, Y: 4},
		{Kind: record.KindUp, Button: core.ButtonLeft, X: 10, Y: 4},
		{Kind: record.KindMove, X: 0, Y: 4},
		{Kind: record.KindMove, X: 0, Y: 49},
		{Kind: record.KindDown, Button: core.ButtonRight, X: 0, Y: 49},
		{Kind: record.KindDown, Button: core.ButtonMiddle, X: 0, Y: 49},
	}
	if len(got) != len(want) {
		t.Fatalf("len %d %+v", len(got), got)
	}
	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Button != want[i].Button || got[i].X != want[i].X || got[i].Y != want[i].Y {
			t.Fatalf("%d %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestScaleAbsToScreen(t *testing.T) {
	if scaleAbs(0, 0, 1000, 1001) != 0 || scaleAbs(1000, 0, 1000, 1001) != 1000 || scaleAbs(500, 0, 1000, 1001) != 500 {
		t.Fatalf("span %d %d %d", scaleAbs(0, 0, 1000, 1001), scaleAbs(1000, 0, 1000, 1001), scaleAbs(500, 0, 1000, 1001))
	}
	if scaleAbs(-10, 0, 100, 101) != 0 || scaleAbs(1000, 0, 100, 101) != 100 {
		t.Fatal("clamp")
	}
	if scaleAbs(150, 100, 200, 0) != 50 {
		t.Fatal(scaleAbs(150, 100, 200, 0))
	}
	if scaleAbs(5, 10, 20, 0) != 0 || scaleAbs(100, 10, 10, 50) != 0 {
		t.Fatal("degenerate")
	}

	p := pointerTrack{w: 101, h: 51}
	dev := &evdevDev{
		hasX: true, hasY: true, hasMTX: true,
		absX:   absAxis{min: 0, max: 1000},
		absY:   absAxis{min: 0, max: 100},
		absMTX: absAxis{min: 0, max: 1000},
	}
	ev, ok := applyEvdevPointer(&p, dev, inputEvent{Type: evAbs, Code: absX, Value: 250})
	if !ok || ev.X != 25 || ev.Y != 0 || ev.Kind != record.KindMove {
		t.Fatalf("%+v %v", ev, ok)
	}
	ev, ok = applyEvdevPointer(&p, dev, inputEvent{Type: evAbs, Code: absY, Value: 100})
	if !ok || ev.X != 25 || ev.Y != 50 {
		t.Fatalf("y %+v", ev)
	}
	ev, ok = applyEvdevPointer(&p, dev, inputEvent{Type: evAbs, Code: absMTX, Value: 0})
	if !ok || ev.X != 0 || ev.Y != 50 {
		t.Fatalf("mt %+v", ev)
	}
}

func TestEvdevChordMatch(t *testing.T) {
	chord, err := core.ParseHotkey("Ctrl+Shift+F12")
	if err != nil {
		t.Fatal(err)
	}
	var mods modifierState
	mods.apply(keyLeftCtrl, 1)
	mods.apply(keyRightShift, 1)
	if !chordHit(chord, keyF12, 1, mods) {
		t.Fatal("expected chord")
	}
	if chordHit(chord, keyF12, 0, mods) || chordHit(chord, keyF12, 2, mods) {
		t.Fatal("release and repeat must not fire")
	}
	mods.apply(keyLeftCtrl, 0)
	if chordHit(chord, keyF12, 1, mods) {
		t.Fatal("ctrl released")
	}
	mods.apply(keyRightCtrl, 1)
	if !chordHit(chord, keyF12, 1, mods) {
		t.Fatal("right ctrl should count")
	}
	mods.apply(keyLeftAlt, 1)
	if chordHit(chord, keyF12, 1, mods) {
		t.Fatal("extra alt")
	}

	plain, err := core.ParseHotkey("F12")
	if err != nil {
		t.Fatal(err)
	}
	var bare modifierState
	if !chordHit(plain, keyF12, 1, bare) {
		t.Fatal("plain F12")
	}
	bare.apply(keyLeftMeta, 1)
	if chordHit(plain, keyF12, 1, bare) {
		t.Fatal("super held")
	}
	bare.apply(keyLeftMeta, 0)
	bare.apply(keyRightMeta, 1)
	super, err := core.ParseHotkey("Super+a")
	if err != nil {
		t.Fatal(err)
	}
	code, ok := evdevKeyCode("a")
	if !ok || code != keyA {
		t.Fatal(code, ok)
	}
	if !chordHit(super, code, 1, bare) {
		t.Fatal("super+a")
	}
}

func TestEvdevKeyCodes(t *testing.T) {
	names := []string{"escape", "return", "tab", "backspace", "delete", "insert", "home", "end", "pageup", "pagedown", "left", "up", "right", "down", "space", "a", "z", "0", "9", "f1", "f10", "f11", "f12", "f24"}
	for _, name := range names {
		if _, ok := evdevKeyCode(name); !ok {
			t.Fatal(name)
		}
	}
	if c, _ := evdevKeyCode("f1"); c != keyF1 {
		t.Fatal(c)
	}
	if c, _ := evdevKeyCode("f12"); c != keyF12 {
		t.Fatal(c)
	}
	if c, _ := evdevKeyCode("f13"); c != keyF13 {
		t.Fatal(c)
	}
	if c, _ := evdevKeyCode("0"); c != key0 {
		t.Fatal(c)
	}
	if _, ok := evdevKeyCode("not-a-key"); ok {
		t.Fatal("unknown key")
	}
}

func TestKeyBitClassification(t *testing.T) {
	bits := make([]byte, evdevKeyBytes)
	if pointerKeyBits(bits) || keyboardKeyBits(bits) {
		t.Fatal("empty mask")
	}
	// BTN_LEFT is 0x110 = 272 → byte 34, bit 0.
	bits[272/8] |= 1 << uint(272%8)
	if !pointerKeyBits(bits) || keyboardKeyBits(bits) {
		t.Fatal("pointer")
	}
	keys := make([]byte, evdevKeyBytes)
	keys[keyF1/8] |= 1 << uint(keyF1%8)
	if !keyboardKeyBits(keys) || pointerKeyBits(keys) {
		t.Fatal("f1")
	}
	onlyF12 := make([]byte, 8)
	if keyboardKeyBits(onlyF12) {
		t.Fatal("short mask")
	}
	wide := make([]byte, evdevKeyBytes)
	wide[keyF12/8] |= 1 << uint(keyF12%8)
	if !keyboardKeyBits(wide) {
		t.Fatal("f12")
	}
	if bitSet(nil, 0) || bitSet(bits, -1) {
		t.Fatal("invalid bit")
	}
}

func TestInputGroupError(t *testing.T) {
	err := errNeedInputGroup()
	if !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	if !strings.Contains(err.Error(), "input group") || !strings.Contains(err.Error(), "/dev/input/event*") {
		t.Fatal(err)
	}
}

func TestPointerSeedUsesScreenEnv(t *testing.T) {
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("CLICK_SPIRIT_BACKEND", "")
	t.Setenv("CLICK_SPIRIT_SCREEN", "1280x800")
	x, y, w, h := pointerSeed()
	if x != 0 || y != 0 || w != 1280 || h != 800 {
		t.Fatalf("%d,%d %dx%d", x, y, w, h)
	}
}

func TestRecordSelectionWithoutX11(t *testing.T) {
	t.Setenv("CLICK_SPIRIT_BACKEND", "")
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	if x11RecordAvailable() || RecordingUsesX11() {
		t.Fatal("XRecord selected without DISPLAY")
	}
	t.Setenv("CLICK_SPIRIT_BACKEND", "uinput")
	t.Setenv("DISPLAY", ":1")
	if x11RecordAvailable() {
		t.Fatal("forced uinput must not open XRecord")
	}
}

func TestHotkeyEvdevWhenDisplayUnset(t *testing.T) {
	t.Setenv("DISPLAY", "")
	src, err := NewHotkey("F12")
	if err != nil || src == nil {
		t.Fatal(err)
	}
	if _, ok := src.(*evdevHotkey); !ok {
		t.Fatalf("%T", src)
	}
	src.Stop()
	src.Stop()
}

func TestNewInputSourceLinuxStop(t *testing.T) {
	if disp := os.Getenv("DISPLAY"); forbiddenSharedDisplay(disp) {
		t.Fatalf("refusing to open shared display %q", disp)
	}
	src, err := NewInputSource()
	if err != nil || src == nil {
		t.Fatal(err)
	}
	src.Stop()
	src.Stop()
}
