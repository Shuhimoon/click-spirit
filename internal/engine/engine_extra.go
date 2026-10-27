package engine

import (
	"encoding/json"
	"errors"
	"image/png"
	"math/rand/v2"
	"os"
	"strings"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/core/imagematch"
	"github.com/click-spirit/click-spirit/core/record"
)

// SetCapturer installs the screenshot source used by image steps.
// When it is nil, Start uses the clicker if it implements core.ScreenCapturer.
func (e *Engine) SetCapturer(c core.ScreenCapturer) {
	e.mu.Lock()
	e.capturer = c
	e.mu.Unlock()
}

// SetHotkeyFactory installs the platform hook used by SetHotkey and by a
// script's emergency_hotkey field. A nil factory leaves those calls unable
// to arm a key; EmergencyStop still works.
func (e *Engine) SetHotkeyFactory(fn func(name string) (core.HotkeySource, error)) {
	e.mu.Lock()
	e.newHotkey = fn
	e.mu.Unlock()
}

// SetInputFactory installs the platform input hook used by StartRecording.
func (e *Engine) SetInputFactory(fn func() (record.InputSource, error)) {
	e.mu.Lock()
	e.newInput = fn
	e.mu.Unlock()
}

// SetRecordOptions sets the conversion used by StopRecording.
// The zero value uses record's defaults (speed 1, 300ms double-click).
func (e *Engine) SetRecordOptions(opt record.Options) {
	e.mu.Lock()
	e.recOpt = opt
	e.mu.Unlock()
}

// SetHotkey arms name as the emergency-stop chord. An empty name disarms it.
// A trigger calls EmergencyStop. The callback must not call SetHotkey.
func (e *Engine) SetHotkey(name string) error {
	name = strings.TrimSpace(name)
	if name != "" {
		if _, err := core.ParseHotkey(name); err != nil {
			return err
		}
	}
	e.hotkeyMu.Lock()
	defer e.hotkeyMu.Unlock()

	e.mu.Lock()
	prev := e.hotkey
	factory := e.newHotkey
	e.hotkey = nil
	e.hotkeyName = ""
	e.mu.Unlock()
	if prev != nil {
		prev.Stop()
	}
	if name == "" {
		return nil
	}
	if factory == nil {
		return errors.New("hotkey source is not available")
	}
	src, err := factory(name)
	if err != nil {
		return err
	}
	if err := src.Start(func() { e.EmergencyStop() }); err != nil {
		src.Stop()
		return err
	}
	e.mu.Lock()
	e.hotkey = src
	e.hotkeyName = name
	e.mu.Unlock()
	return nil
}

// EmergencyStop emits an emergency_stop event and cancels the active run.
func (e *Engine) EmergencyStop() {
	e.onRunnerEvent(core.Event{Kind: core.EventEmergencyStop, StepIndex: -1})
	e.Stop()
}

// StartRecording begins capturing pointer events from the input factory.
func (e *Engine) StartRecording() error {
	e.mu.Lock()
	factory := e.newInput
	e.mu.Unlock()
	if factory == nil {
		return errors.New("input recording is not available")
	}
	src, err := factory()
	if err != nil {
		return err
	}
	return e.StartRecordingWith(src)
}

// StartRecordingWith records from src until StopRecording.
func (e *Engine) StartRecordingWith(src record.InputSource) error {
	if src == nil {
		return errors.New("nil input source")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.rec != nil && e.rec.Active() {
		return errors.New("already recording")
	}
	rec := record.New()
	if err := rec.Start(src); err != nil {
		return err
	}
	e.rec = rec
	return nil
}

// AddRecordingEvent appends a sample to the active recording.
func (e *Engine) AddRecordingEvent(ev record.Event) {
	e.mu.Lock()
	rec := e.rec
	e.mu.Unlock()
	if rec != nil {
		rec.Add(ev)
	}
}

// StopRecording ends the recording and returns the script JSON.
func (e *Engine) StopRecording() (string, error) {
	e.mu.Lock()
	rec := e.rec
	opt := e.recOpt
	e.rec = nil
	e.mu.Unlock()
	if rec == nil {
		return "", errors.New("not recording")
	}
	rec.Stop()
	script, err := record.ToScript(rec.Events(), opt)
	if err != nil {
		return "", err
	}
	b, err := script.Marshal()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// MatchImage searches the current capture for the PNG at templatePath.
// The result is JSON {"found","x","y","score","error"}. x and y are the
// center of the match when found.
func (e *Engine) MatchImage(templatePath string, threshold float64) string {
	if strings.TrimSpace(templatePath) == "" {
		return imagematch.Hit{Error: "empty template path"}.JSON()
	}
	img, err := e.Capture()
	if err != nil {
		return (imagematch.Hit{Error: err.Error()}).JSON()
	}
	f, err := os.Open(templatePath)
	if err != nil {
		return (imagematch.Hit{Error: err.Error()}).JSON()
	}
	defer f.Close()
	templ, err := png.Decode(f)
	if err != nil {
		return (imagematch.Hit{Error: err.Error()}).JSON()
	}
	return imagematch.MatchImages(img, templ, threshold).JSON()
}

// RandomPointPreview returns JSON [[x,y],...] with n samples inside the disc.
func (e *Engine) RandomPointPreview(cx, cy, r, n int) (string, error) {
	if cx < 0 || cy < 0 || r < 0 {
		return "", errors.New("cx, cy, and r must be >= 0")
	}
	if n <= 0 {
		n = 1
	}
	if n > 4096 {
		n = 4096
	}
	pts := make([][2]int, n)
	for i := range pts {
		pts[i][0], pts[i][1] = core.PointInDisc(cx, cy, r, rand.Float64(), rand.Float64())
	}
	b, err := json.Marshal(pts)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
