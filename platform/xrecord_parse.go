//go:build linux

package platform

import (
	"unsafe"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/core/record"
)

// xEvent is 32 bytes (X11/Xproto.h sz_xEvent). Device events use
// xEvent.u.keyButtonPointer, whose fields are CARD32/INT16 in protocol
// order, not the host's unsigned long:
//
//	offset 0  BYTE  type
//	offset 1  BYTE  detail (button number for ButtonPress / ButtonRelease)
//	offset 4  CARD32 time
//	offset 8  CARD32 root
//	offset 12 CARD32 event
//	offset 16 CARD32 child
//	offset 20 INT16 rootX
//	offset 22 INT16 rootY
const (
	xEventLen = 32

	xOffType   = 0
	xOffDetail = 1
	xOffRootX  = 20
	xOffRootY  = 22

	xTypeButtonPress   = 4
	xTypeButtonRelease = 5
	xTypeMotionNotify  = 6
)

// parseXRecordServerData decodes one XRecord FromServer payload into pointer
// events. swapped is XRecordInterceptData.client_swapped: the bytes are in
// the opposite endianness from this process. Scroll buttons 4–7 are dropped.
// The caller stamps Time with time.Now at delivery.
func parseXRecordServerData(data []byte, swapped bool) []record.Event {
	var out []record.Event
	for len(data) >= xEventLen {
		if ev, ok := parseXRecordEvent(data[:xEventLen], swapped); ok {
			out = append(out, ev)
		}
		data = data[xEventLen:]
	}
	return out
}

func parseXRecordEvent(b []byte, swapped bool) (record.Event, bool) {
	if len(b) < xEventLen {
		return record.Event{}, false
	}
	typ := b[xOffType] & 0x7f
	x := int(xInt16(b, xOffRootX, swapped))
	y := int(xInt16(b, xOffRootY, swapped))
	switch typ {
	case xTypeButtonPress, xTypeButtonRelease:
		button, ok := xRecordButton(b[xOffDetail])
		if !ok {
			return record.Event{}, false
		}
		kind := record.KindUp
		if typ == xTypeButtonPress {
			kind = record.KindDown
		}
		return record.Event{Kind: kind, Button: button, X: x, Y: y}, true
	case xTypeMotionNotify:
		return record.Event{Kind: record.KindMove, X: x, Y: y}, true
	default:
		return record.Event{}, false
	}
}

// xRecordButton maps X button numbers. 1/2/3 are left/middle/right.
// 4–7 are the scroll wheel and are ignored, as are any other buttons.
func xRecordButton(detail byte) (core.Button, bool) {
	switch detail {
	case 1:
		return core.ButtonLeft, true
	case 2:
		return core.ButtonMiddle, true
	case 3:
		return core.ButtonRight, true
	default:
		return "", false
	}
}

func xInt16(b []byte, off int, swapped bool) int16 {
	le := hostLittleEndian()
	if swapped {
		le = !le
	}
	var u uint16
	if le {
		u = uint16(b[off]) | uint16(b[off+1])<<8
	} else {
		u = uint16(b[off])<<8 | uint16(b[off+1])
	}
	return int16(u)
}

func hostLittleEndian() bool {
	var probe uint16 = 1
	return (*[2]byte)(unsafe.Pointer(&probe))[0] == 1
}
