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

// Step is one click in a script.
// Clicks of 0 means one click. IntervalMs of 0 means the script default.
type Step struct {
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Button     Button `json:"button"`
	Clicks     int    `json:"clicks"`
	IntervalMs int    `json:"interval_ms"`
	HoldMs     int    `json:"hold_ms"`
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

// Script is a click script. Repeat of 0 means loop forever.
type Script struct {
	Version           int    `json:"version"`
	Name              string `json:"name"`
	Steps             []Step `json:"steps"`
	Repeat            int    `json:"repeat"`
	DefaultIntervalMs int    `json:"default_interval_ms"`
	LoopIntervalMs    int    `json:"loop_interval_ms"`
	StartDelayMs      int    `json:"start_delay_ms"`
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
	if st.IntervalMs < 0 || st.HoldMs < 0 {
		return fmt.Errorf("step %d: negative value", i)
	}
	if !st.Button.Valid() {
		return fmt.Errorf("step %d: invalid button %q", i, st.Button)
	}
	if st.Clicks < 0 || st.Clicks > 3 {
		return fmt.Errorf("step %d: clicks must be 0..3", i)
	}
	return nil
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
