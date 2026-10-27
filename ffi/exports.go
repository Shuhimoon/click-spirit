//go:build cgo

package main

/*
#include <stdlib.h>
*/
import "C"

import (
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

//export CS_FreeString
func CS_FreeString(p *C.char) {
	if p != nil {
		C.free(unsafe.Pointer(p))
	}
}

func main() {}
