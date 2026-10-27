package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Button is a mouse button name stored in a script.
type Button string

const (
	ButtonLeft   Button = "left"
	ButtonRight  Button = "right"
	ButtonMiddle Button = "middle"
)

// Valid reports whether b is left, right, or middle.
func (b Button) Valid() bool {
	switch b {
	case ButtonLeft, ButtonRight, ButtonMiddle:
		return true
	default:
		return false
	}
}

// Target kinds. An empty kind is a point at the step's X, Y.
const (
	TargetPoint  = "point"
	TargetCircle = "circle"
	TargetImage  = "image"
)

// What to do when an image template is not on screen.
// Empty means skip.
const (
	OnNotFoundSkip = "skip"
	OnNotFoundWait = "wait"
	OnNotFoundStop = "stop"
)

// DefaultImageThreshold is used when an image target omits threshold or sets 0.
const DefaultImageThreshold = 0.9

// Region is a search rectangle in capture pixels. Origin is the top-left of the capture.
type Region struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Target is an optional click location. Old scripts omit it and click X, Y.
type Target struct {
	Kind        string  `json:"kind"`
	CX          int     `json:"cx,omitempty"`
	CY          int     `json:"cy,omitempty"`
	R           int     `json:"r,omitempty"`
	Template    string  `json:"template,omitempty"`
	TemplateB64 string  `json:"template_b64,omitempty"`
	Threshold   float64 `json:"threshold,omitempty"`
	Region      *Region `json:"region,omitempty"`
	OffsetX     int     `json:"offset_x,omitempty"`
	OffsetY     int     `json:"offset_y,omitempty"`
	OnNotFound  string  `json:"on_not_found,omitempty"`
	TimeoutMs   int     `json:"timeout_ms,omitempty"`
	PollMs      int     `json:"poll_ms,omitempty"`
}

// KindOrPoint returns the target kind. Empty means a point.
func (t Target) KindOrPoint() string {
	if t.Kind == "" {
		return TargetPoint
	}
	return t.Kind
}

// EffectiveThreshold returns the image-match threshold. Zero means the default.
func (t Target) EffectiveThreshold() float64 {
	if t.Threshold <= 0 {
		return DefaultImageThreshold
	}
	return t.Threshold
}

// NotFoundOrSkip returns the image miss behavior. Empty means skip.
func (t Target) NotFoundOrSkip() string {
	if t.OnNotFound == "" {
		return OnNotFoundSkip
	}
	return t.OnNotFound
}

// Step is one click in a script.
// Clicks of 0 means one click. IntervalMs of 0 means the script default.
// Repeat of 0 or 1 runs the step once; a larger Repeat runs it that many
// times consecutively. The interval is applied after every repetition.
type Step struct {
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Button     Button  `json:"button"`
	Clicks     int     `json:"clicks"`
	IntervalMs int     `json:"interval_ms"`
	HoldMs     int     `json:"hold_ms"`
	Repeat     int     `json:"repeat,omitempty"`
	Target     *Target `json:"target,omitempty"`
}

// target returns a copy of the step target. A nil target is a point.
func (s Step) target() Target {
	if s.Target == nil {
		return Target{}
	}
	return *s.Target
}

// EffectiveClicks returns the click count to perform. Zero means one.
func (s Step) EffectiveClicks() int {
	if s.Clicks == 0 {
		return 1
	}
	return s.Clicks
}

// EffectiveIntervalMs returns the delay after this step.
// Zero uses defaultMs, which is the script's default interval.
func (s Step) EffectiveIntervalMs(defaultMs int) int {
	if s.IntervalMs == 0 {
		return defaultMs
	}
	return s.IntervalMs
}

// EffectiveRepeat returns how many times this step runs before the next one.
// Zero and one both mean once.
func (s Step) EffectiveRepeat() int {
	if s.Repeat <= 1 {
		return 1
	}
	return s.Repeat
}

// Script is a click script. Repeat of 0 means loop forever.
type Script struct {
	Version           int    `json:"version"`
	Name              string `json:"name"`
	Steps             []Step `json:"steps"`
	Repeat            int    `json:"repeat"`
	DefaultIntervalMs int    `json:"default_interval_ms"`
	LoopIntervalMs    int    `json:"loop_interval_ms"`
	StartDelayMs      int    `json:"start_delay_ms"`
	// EmergencyHotkey is an optional chord such as "F12" or "Ctrl+Shift+Esc".
	// The engine arms it when a hotkey backend is configured. Empty leaves the
	// current hotkey alone.
	EmergencyHotkey string `json:"emergency_hotkey,omitempty"`
}

// Validate checks the script. Steps must be non-empty. Numeric fields must
// be non-negative. Each button must be valid, X and Y must be >= 0, and
// clicks must be in 0..3 (0 means one click at run time).
func (s *Script) Validate() error {
	if s == nil {
		return errors.New("nil script")
	}
	if len(s.Steps) == 0 {
		return errors.New("steps must be non-empty")
	}
	if s.Version < 0 || s.Repeat < 0 || s.DefaultIntervalMs < 0 || s.LoopIntervalMs < 0 || s.StartDelayMs < 0 {
		return errors.New("negative value")
	}
	if s.EmergencyHotkey != "" {
		if _, err := ParseHotkey(s.EmergencyHotkey); err != nil {
			return fmt.Errorf("emergency_hotkey: %w", err)
		}
	}
	for i, st := range s.Steps {
		if err := validateStep(i, st); err != nil {
			return err
		}
	}
	return nil
}

func validateStep(i int, st Step) error {
	if st.X < 0 || st.Y < 0 {
		return fmt.Errorf("step %d: X and Y must be >= 0", i)
	}
	if st.IntervalMs < 0 || st.HoldMs < 0 || st.Repeat < 0 {
		return fmt.Errorf("step %d: negative value", i)
	}
	if !st.Button.Valid() {
		return fmt.Errorf("step %d: invalid button %q", i, st.Button)
	}
	if st.Clicks < 0 || st.Clicks > 3 {
		return fmt.Errorf("step %d: clicks must be 0..3", i)
	}
	return validateTarget(i, st.Target)
}

func validateTarget(i int, t *Target) error {
	if t == nil {
		return nil
	}
	switch t.Kind {
	case "", TargetPoint:
		return nil
	case TargetCircle:
		if t.CX < 0 || t.CY < 0 || t.R < 0 {
			return fmt.Errorf("step %d: circle cx, cy, and r must be >= 0", i)
		}
		return nil
	case TargetImage:
		if t.Template == "" && t.TemplateB64 == "" {
			return fmt.Errorf("step %d: image target needs template or template_b64", i)
		}
		if t.Threshold < 0 || t.Threshold > 1 {
			return fmt.Errorf("step %d: threshold must be 0..1", i)
		}
		switch t.OnNotFound {
		case "", OnNotFoundSkip, OnNotFoundWait, OnNotFoundStop:
		default:
			return fmt.Errorf("step %d: on_not_found must be skip, wait, or stop", i)
		}
		if t.TimeoutMs < 0 || t.PollMs < 0 {
			return fmt.Errorf("step %d: negative timeout or poll", i)
		}
		if t.Region != nil {
			r := t.Region
			if r.X < 0 || r.Y < 0 || r.W < 0 || r.H < 0 {
				return fmt.Errorf("step %d: region must be >= 0", i)
			}
		}
		return nil
	default:
		return fmt.Errorf("step %d: unknown target kind %q", i, t.Kind)
	}
}

// Marshal returns indented JSON using the snake_case field tags.
func (s *Script) Marshal() ([]byte, error) {
	if s == nil {
		return nil, errors.New("nil script")
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Unmarshal parses script JSON.
func Unmarshal(data []byte) (*Script, error) {
	var s Script
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Save writes the script as indented JSON.
func (s *Script) Save(path string) error {
	b, err := s.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Load reads a script from path. It does not validate it.
func Load(path string) (*Script, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Unmarshal(b)
}

// NewSample returns a small script that shows every field.
func NewSample() *Script {
	return &Script{
		Version:           1,
		Name:              "示範腳本",
		Repeat:            1,
		DefaultIntervalMs: 100,
		LoopIntervalMs:    500,
		StartDelayMs:      200,
		Steps: []Step{
			{X: 100, Y: 200, Button: ButtonLeft, Clicks: 1, IntervalMs: 50, HoldMs: 0},
			{X: 300, Y: 400, Button: ButtonRight, Clicks: 2, IntervalMs: 0, HoldMs: 30},
		},
	}
}
