//go:build cgo

package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"image/png"
	"os"
	"sync"
	"unsafe"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/internal/engine"
	"github.com/click-spirit/click-spirit/platform"
)

// Strings returned to C are allocated with malloc. The caller frees them
// with CS_FreeString. An empty string is a non-NULL pointer and must be freed too.

var (
	engineOnce sync.Once
	sharedEng  *engine.Engine
)

func shared() *engine.Engine {
	engineOnce.Do(func() {
		c, err := platform.New()
		sharedEng = engine.New(c, err)
		sharedEng.SetHotkeyFactory(platform.NewHotkey)
		sharedEng.SetInputFactory(platform.NewInputSource)
		if c != nil {
			if ov, ovErr := platform.NewClickOverlay(); ovErr == nil {
				sharedEng.SetOverlay(ov)
			}
			if p, pErr := platform.NewScreenPicker(); pErr == nil {
				sharedEng.SetPicker(p)
			}
			if b, bErr := platform.NewBubble(); bErr == nil {
				sharedEng.SetBubble(b)
			}
			if v, vErr := platform.NewVideoRecorder(); vErr == nil {
				sharedEng.SetVideoRecorder(v)
			}
		}
	})
	return sharedEng
}

func cString(s string) *C.char {
	return C.CString(s)
}

func goString(p *C.char) string {
	if p == nil {
		return ""
	}
	return C.GoString(p)
}

//export CS_Version
func CS_Version() *C.char {
	return cString("click-spirit 0.1.0")
}

//export CS_SampleScript
func CS_SampleScript() *C.char {
	b, err := core.NewSample().Marshal()
	if err != nil {
		return cString("")
	}
	return cString(string(b))
}

//export CS_Validate
func CS_Validate(json *C.char) *C.char {
	s, err := core.Unmarshal([]byte(goString(json)))
	if err != nil {
		return cString(err.Error())
	}
	if err := s.Validate(); err != nil {
		return cString(err.Error())
	}
	return cString("")
}

//export CS_Start
func CS_Start(json *C.char) *C.char {
	if err := shared().Start(goString(json)); err != nil {
		return cString(err.Error())
	}
	return cString("")
}

//export CS_Stop
func CS_Stop() {
	shared().Stop()
}

//export CS_IsRunning
func CS_IsRunning() C.int {
	if shared().Running() {
		return 1
	}
	return 0
}

//export CS_CursorPos
func CS_CursorPos(x, y *C.int) C.int {
	xx, yy, ok := shared().CursorPos()
	if !ok {
		return 0
	}
	if x != nil {
		*x = C.int(xx)
	}
	if y != nil {
		*y = C.int(yy)
	}
	return 1
}

//export CS_PollEvent
func CS_PollEvent() *C.char {
	s, ok := shared().PollEvent()
	if !ok {
		return cString("")
	}
	return cString(s)
}

//export CS_AccessibilityTrusted
func CS_AccessibilityTrusted() C.int {
	if platform.AccessibilityTrusted() {
		return 1
	}
	return 0
}

//export CS_Backend
func CS_Backend() *C.char {
	return cString(platform.Name())
}

//export CS_CaptureScreenPNG
func CS_CaptureScreenPNG(path *C.char) *C.char {
	p := goString(path)
	if p == "" {
		return cString("empty path")
	}
	img, err := shared().Capture()
	if err != nil {
		return cString(err.Error())
	}
	f, err := os.Create(p)
	if err != nil {
		return cString(err.Error())
	}
	encErr := png.Encode(f, img)
	closeErr := f.Close()
	if encErr != nil {
		return cString(encErr.Error())
	}
	if closeErr != nil {
		return cString(closeErr.Error())
	}
	return cString("")
}

//export CS_EmergencyStop
func CS_EmergencyStop() {
	shared().EmergencyStop()
}

//export CS_SetHotkey
func CS_SetHotkey(name *C.char) *C.char {
	if err := shared().SetHotkey(goString(name)); err != nil {
		return cString(err.Error())
	}
	return cString("")
}

//export CS_StartRecording
func CS_StartRecording() *C.char {
	if err := shared().StartRecording(); err != nil {
		return cString(jsonErr(err))
	}
	return cString(`{"ok":true}`)
}

//export CS_StopRecording
func CS_StopRecording() *C.char {
	s, err := shared().StopRecording()
	if err != nil {
		return cString(jsonErr(err))
	}
	return cString(s)
}

//export CS_MatchImage
func CS_MatchImage(templatePath *C.char, threshold C.double) *C.char {
	return cString(shared().MatchImage(goString(templatePath), float64(threshold)))
}

func jsonErr(err error) string {
	b, mErr := json.Marshal(struct {
		Error string `json:"error"`
	}{Error: err.Error()})
	if mErr != nil {
		return `{"error":"unknown"}`
	}
	return string(b)
}

// CS_SetShowClicks turns the run-time click overlay on (non-zero) or off.
//
//export CS_SetShowClicks
func CS_SetShowClicks(on C.int) {
	shared().SetShowClicks(on != 0)
}

// CS_HasClickOverlay returns 1 when this backend can draw the run-time click
// overlay (Linux X11 today). Otherwise the app shows clicks in its own window.
//
//export CS_HasClickOverlay
func CS_HasClickOverlay() C.int {
	if shared().HasOverlay() {
		return 1
	}
	return 0
}

//export CS_FreeString
func CS_FreeString(p *C.char) {
	if p != nil {
		C.free(unsafe.Pointer(p))
	}
}

func main() {}

// CS_StartPick opens the native on-screen picker ("point" or "circle") and
// returns at once: "" when started, otherwise an error message. The result
// arrives as one polled event: pick (x, y, r), pick_canceled, or pick_error.
//
//export CS_StartPick
func CS_StartPick(mode *C.char) *C.char {
	if err := shared().StartPick(core.PickMode(goString(mode))); err != nil {
		return cString(err.Error())
	}
	return cString("")
}

// CS_HasScreenPicker returns 1 when the native on-screen picker exists.
//
//export CS_HasScreenPicker
func CS_HasScreenPicker() C.int {
	if shared().HasPicker() {
		return 1
	}
	return 0
}

// CS_HasBubble returns 1 when the floating bubble (小浮窗) exists here.
//
//export CS_HasBubble
func CS_HasBubble() C.int {
	if shared().HasBubble() {
		return 1
	}
	return 0
}

// CS_SetBubbleVisible shows (1) or hides (0) the floating bubble. Presses on
// its buttons arrive as "bubble" events with message record/start/stop.
//
//export CS_SetBubbleVisible
func CS_SetBubbleVisible(on C.int) {
	shared().ShowBubble(on != 0)
}

// CS_HasVideoRecorder returns 1 when screen video recording works here.
//
//export CS_HasVideoRecorder
func CS_HasVideoRecorder() C.int {
	if shared().HasVideoRecorder() {
		return 1
	}
	return 0
}

// CS_StartScreenRecording starts 錄影轉腳本: clicks with timestamps plus a
// screen video at videoPath when a backend exists. Returns "" or an error.
//
//export CS_StartScreenRecording
func CS_StartScreenRecording(videoPath *C.char) *C.char {
	if err := shared().StartScreenRecording(goString(videoPath)); err != nil {
		return cString(err.Error())
	}
	return cString("")
}

// CS_StopScreenRecording returns JSON {video, note, clicks:[{x,y,button,at_ms,hold_ms}], error}.
//
//export CS_StopScreenRecording
func CS_StopScreenRecording() *C.char {
	return cString(shared().StopScreenRecording())
}

// CS_VideoFrame writes a PNG thumbnail of video at atMs. Returns "" or an error.
//
//export CS_VideoFrame
func CS_VideoFrame(video *C.char, atMs C.longlong, out *C.char, width C.int) *C.char {
	if err := platform.VideoFrame(goString(video), int64(atMs), goString(out), int(width)); err != nil {
		return cString(err.Error())
	}
	return cString("")
}

// CS_ClicksToScript turns reviewed clicks JSON into script JSON. On failure
// it returns "error: ...".
//
//export CS_ClicksToScript
func CS_ClicksToScript(clicks, name *C.char) *C.char {
	s, err := engine.ClicksToScript(goString(clicks), goString(name))
	if err != nil {
		return cString("error: " + err.Error())
	}
	return cString(s)
}
