//go:build linux

package platform

import (
	"testing"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/core/record"
)

func TestParseXRecordButtonsAndMotion(t *testing.T) {
	batch := append([]byte{}, putXEvent(xTypeButtonPress, 1, 80, 90, false)...)
	batch = append(batch, putXEvent(xTypeButtonRelease, 1, 80, 91, false)...)
	batch = append(batch, putXEvent(xTypeMotionNotify, 0, -4, 12, false)...)
	batch = append(batch, putXEvent(xTypeButtonPress, 4, 1, 2, false)...)
	batch = append(batch, putXEvent(xTypeButtonPress, 7, 3, 4, false)...)
	batch = append(batch, putXEvent(xTypeButtonPress|0x80, 2, 5, 6, false)...)
	batch = append(batch, putXEvent(xTypeButtonRelease, 3, 7, 8, false)...)
	batch = append(batch, 0x01, 0x02) // short tail is ignored

	got := parseXRecordServerData(batch, false)
	want := []record.Event{
		{Kind: record.KindDown, Button: core.ButtonLeft, X: 80, Y: 90},
		{Kind: record.KindUp, Button: core.ButtonLeft, X: 80, Y: 91},
		{Kind: record.KindMove, X: -4, Y: 12},
		{Kind: record.KindDown, Button: core.ButtonMiddle, X: 5, Y: 6},
		{Kind: record.KindUp, Button: core.ButtonRight, X: 7, Y: 8},
	}
	if len(got) != len(want) {
		t.Fatalf("len %d %+v", len(got), got)
	}
	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Button != want[i].Button || got[i].X != want[i].X || got[i].Y != want[i].Y {
			t.Fatalf("event %d %+v want %+v", i, got[i], want[i])
		}
		if !got[i].Time.IsZero() {
			t.Fatal("parser should leave time for the delivery site")
		}
	}
}

func TestParseXRecordSwappedEndian(t *testing.T) {
	raw := putXEvent(xTypeButtonPress, 3, 300, -20, true)
	got := parseXRecordServerData(raw, true)
	if len(got) != 1 || got[0].Button != core.ButtonRight || got[0].X != 300 || got[0].Y != -20 || got[0].Kind != record.KindDown {
		t.Fatalf("%+v", got)
	}
}

func putXEvent(typ, detail byte, x, y int16, swapped bool) []byte {
	b := make([]byte, xEventLen)
	b[xOffType] = typ
	b[xOffDetail] = detail
	putXInt16(b, xOffRootX, x, swapped)
	putXInt16(b, xOffRootY, y, swapped)
	return b
}

func putXInt16(b []byte, off int, v int16, swapped bool) {
	u := uint16(v)
	le := hostLittleEndian()
	if swapped {
		le = !le
	}
	if le {
		b[off] = byte(u)
		b[off+1] = byte(u >> 8)
		return
	}
	b[off] = byte(u >> 8)
	b[off+1] = byte(u)
}
