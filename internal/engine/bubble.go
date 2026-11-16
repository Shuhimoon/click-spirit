package engine

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/core/record"
)

// EventBubble is queued when a bubble button is pressed; message is the
// action (record, start, stop).
const EventBubble = "bubble"

// SetBubble installs the floating bubble. Nil removes it.
func (e *Engine) SetBubble(b core.Bubble) {
	e.mu.Lock()
	e.bubble = b
	e.mu.Unlock()
	if b != nil {
		b.SetOnAction(func(action string) {
			raw, _ := json.Marshal(eventPayload{Kind: EventBubble, Step: -1, Message: action})
			e.mu.Lock()
			e.pushLocked(string(raw))
			e.mu.Unlock()
		})
	}
}

// HasBubble reports whether a floating bubble exists on this platform.
func (e *Engine) HasBubble() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.bubble != nil
}

// ShowBubble shows or hides the floating bubble.
func (e *Engine) ShowBubble(on bool) {
	e.mu.Lock()
	b := e.bubble
	e.mu.Unlock()
	if b != nil {
		b.SetVisible(on)
		e.syncBubble()
	}
}

// inBubble is the runner's Exclude hook and the recording filter.
func (e *Engine) inBubble(x, y int) bool {
	e.mu.Lock()
	b := e.bubble
	e.mu.Unlock()
	if b == nil {
		return false
	}
	r, ok := b.Rect()
	return ok && r.Contains(x, y)
}

func (e *Engine) syncBubble() {
	e.mu.Lock()
	running := e.starting || (e.runner != nil && e.runner.Running())
	e.mu.Unlock()
	e.syncBubbleRunning(running)
}

func (e *Engine) syncBubbleRunning(running bool) {
	e.mu.Lock()
	b := e.bubble
	recording := e.rec != nil && e.rec.Active()
	e.mu.Unlock()
	if b != nil {
		b.SetState(recording, running)
	}
}

// filteredSource drops presses on the bubble before they reach the recorder.
type filteredSource struct {
	src    record.InputSource
	filter *record.FilterExcluded
}

func (f *filteredSource) Start(fn func(record.Event)) error {
	return f.src.Start(func(ev record.Event) {
		if f.filter.Keep(ev) {
			fn(ev)
		}
	})
}

func (f *filteredSource) Stop() { f.src.Stop() }

// SetVideoRecorder installs the screen video backend (錄影轉腳本).
func (e *Engine) SetVideoRecorder(v core.VideoRecorder) {
	e.mu.Lock()
	e.video = v
	e.mu.Unlock()
}

// ScreenRecording is the result of StopScreenRecording.
type ScreenRecording struct {
	Video  string              `json:"video"`
	Note   string              `json:"note,omitempty"`
	Clicks []record.TimedClick `json:"clicks"`
	Error  string              `json:"error,omitempty"`
	// ScreenW/ScreenH are the recorded screen size when known (0 otherwise).
	ScreenW int `json:"screen_w,omitempty"`
	ScreenH int `json:"screen_h,omitempty"`
}

// screenSizer is optionally implemented by a VideoRecorder.
type screenSizer interface{ ScreenSize() (int, int) }

// StartScreenRecording records click positions with timestamps and, when a
// video backend exists, the screen to videoPath. Without a backend (or when
// it fails) only the clicks are recorded and the result says so.
func (e *Engine) StartScreenRecording(videoPath string) error {
	if err := e.StartRecording(); err != nil {
		return err
	}
	e.mu.Lock()
	v := e.video
	e.mu.Unlock()
	note := ""
	path := ""
	switch {
	case v == nil:
		note = "此平台沒有錄影後端（Linux 需要 ffmpeg），只記錄點擊"
	case strings.TrimSpace(videoPath) == "":
		note = "沒有指定影片路徑，只記錄點擊"
	default:
		if err := v.Start(videoPath); err != nil {
			note = "無法錄影：" + err.Error() + "；只記錄點擊"
		} else {
			path = videoPath
		}
	}
	e.mu.Lock()
	e.videoPath, e.videoNote, e.videoStart = path, note, time.Now()
	e.mu.Unlock()
	return nil
}

// StopScreenRecording stops the video and the click capture and returns
// JSON ScreenRecording with clicks timed from the start of the video.
func (e *Engine) StopScreenRecording() string {
	e.mu.Lock()
	rec, v, path, note, start := e.rec, e.video, e.videoPath, e.videoNote, e.videoStart
	e.rec = nil
	e.videoPath = ""
	e.mu.Unlock()
	out := ScreenRecording{Video: path, Note: note}
	if sz, ok := v.(screenSizer); ok && v != nil {
		out.ScreenW, out.ScreenH = sz.ScreenSize()
	}
	if rec == nil {
		out.Error = "not recording"
	} else {
		rec.Stop()
		out.Clicks = record.ClicksFromEvents(rec.Events(), start, nil)
	}
	if out.Clicks == nil {
		out.Clicks = []record.TimedClick{}
	}
	if path != "" && v != nil {
		if err := v.Stop(); err != nil {
			out.Note = strings.TrimSpace(out.Note + " 影片結束時出錯：" + err.Error())
		}
	}
	e.syncBubble()
	b, _ := json.Marshal(out)
	return string(b)
}

// ClicksToScript converts reviewed clicks (JSON array of TimedClick) into
// script JSON: one point step per click, gaps become intervals.
func ClicksToScript(clicksJSON, name string) (string, error) {
	var clicks []record.TimedClick
	if err := json.Unmarshal([]byte(clicksJSON), &clicks); err != nil {
		return "", errors.New("clicks JSON: " + err.Error())
	}
	s, err := record.ScriptFromClicks(clicks, name, nil, 500)
	if err != nil {
		return "", err
	}
	b, err := s.Marshal()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// HasVideoRecorder reports whether screen video recording is available.
func (e *Engine) HasVideoRecorder() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.video != nil
}
