package core

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	base := NewSample()
	cases := []struct {
		name    string
		mutate  func(*Script)
		wantErr bool
	}{
		{name: "sample", wantErr: false},
		{name: "clicks zero", mutate: func(s *Script) { s.Steps[0].Clicks = 0 }, wantErr: false},
		{name: "clicks three", mutate: func(s *Script) { s.Steps[0].Clicks = 3 }, wantErr: false},
		{name: "version zero", mutate: func(s *Script) { s.Version = 0 }, wantErr: false},
		{name: "repeat zero", mutate: func(s *Script) { s.Repeat = 0 }, wantErr: false},
		{name: "empty steps", mutate: func(s *Script) { s.Steps = nil }, wantErr: true},
		{name: "negative version", mutate: func(s *Script) { s.Version = -1 }, wantErr: true},
		{name: "negative repeat", mutate: func(s *Script) { s.Repeat = -1 }, wantErr: true},
		{name: "negative default interval", mutate: func(s *Script) { s.DefaultIntervalMs = -1 }, wantErr: true},
		{name: "negative loop interval", mutate: func(s *Script) { s.LoopIntervalMs = -1 }, wantErr: true},
		{name: "negative start delay", mutate: func(s *Script) { s.StartDelayMs = -1 }, wantErr: true},
		{name: "negative x", mutate: func(s *Script) { s.Steps[0].X = -1 }, wantErr: true},
		{name: "negative y", mutate: func(s *Script) { s.Steps[0].Y = -1 }, wantErr: true},
		{name: "negative interval", mutate: func(s *Script) { s.Steps[0].IntervalMs = -1 }, wantErr: true},
		{name: "negative hold", mutate: func(s *Script) { s.Steps[0].HoldMs = -1 }, wantErr: true},
		{name: "bad button", mutate: func(s *Script) { s.Steps[0].Button = "LEFT" }, wantErr: true},
		{name: "empty button", mutate: func(s *Script) { s.Steps[0].Button = "" }, wantErr: true},
		{name: "clicks four", mutate: func(s *Script) { s.Steps[0].Clicks = 4 }, wantErr: true},
		{name: "clicks negative", mutate: func(s *Script) { s.Steps[0].Clicks = -1 }, wantErr: true},
		{name: "repeat zero step", mutate: func(s *Script) { s.Steps[0].Repeat = 0 }, wantErr: false},
		{name: "repeat many", mutate: func(s *Script) { s.Steps[0].Repeat = 4 }, wantErr: false},
		{name: "negative step repeat", mutate: func(s *Script) { s.Steps[0].Repeat = -1 }, wantErr: true},
		{name: "circle", mutate: func(s *Script) {
			s.Steps[0].Target = &Target{Kind: TargetCircle, CX: 10, CY: 12, R: 4}
		}, wantErr: false},
		{name: "negative circle", mutate: func(s *Script) {
			s.Steps[0].Target = &Target{Kind: TargetCircle, CX: -1, CY: 0, R: 1}
		}, wantErr: true},
		{name: "image", mutate: func(s *Script) {
			s.Steps[0].Target = &Target{Kind: TargetImage, Template: "a.png", Threshold: 0.5, OnNotFound: OnNotFoundWait}
		}, wantErr: false},
		{name: "image missing template", mutate: func(s *Script) {
			s.Steps[0].Target = &Target{Kind: TargetImage}
		}, wantErr: true},
		{name: "bad kind", mutate: func(s *Script) {
			s.Steps[0].Target = &Target{Kind: "rect"}
		}, wantErr: true},
		{name: "bad on_not_found", mutate: func(s *Script) {
			s.Steps[0].Target = &Target{Kind: TargetImage, Template: "a.png", OnNotFound: "retry"}
		}, wantErr: true},
		{name: "threshold too high", mutate: func(s *Script) {
			s.Steps[0].Target = &Target{Kind: TargetImage, Template: "a.png", Threshold: 1.1}
		}, wantErr: true},
		{name: "hotkey", mutate: func(s *Script) { s.EmergencyHotkey = "Ctrl+Shift+Esc" }, wantErr: false},
		{name: "bad hotkey", mutate: func(s *Script) { s.EmergencyHotkey = "Nope" }, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := cloneScript(base)
			if tc.mutate != nil {
				tc.mutate(s)
			}
			err := s.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := (*Script)(nil).Validate(); err == nil {
		t.Fatal("nil script")
	}
}

func TestButtonValid(t *testing.T) {
	if !ButtonLeft.Valid() || !ButtonRight.Valid() || !ButtonMiddle.Valid() {
		t.Fatal("expected known buttons")
	}
	if Button("other").Valid() {
		t.Fatal("unknown button")
	}
}

func TestJSONRoundTripAndSaveLoad(t *testing.T) {
	in := NewSample()
	in.Steps[1].Clicks = 0
	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, key := range []string{
		"version", "name", "steps", "repeat",
		"default_interval_ms", "loop_interval_ms", "start_delay_ms",
		"interval_ms", "hold_ms", "clicks", "button",
	} {
		if !strings.Contains(text, `"`+key+`"`) {
			t.Fatalf("missing key %s in %s", key, text)
		}
	}
	if strings.Contains(text, "IntervalMs") || strings.Contains(text, "DefaultIntervalMs") {
		t.Fatalf("Go field name leaked into JSON: %s", text)
	}
	if strings.Contains(text, `"target"`) || strings.Contains(text, "emergency_hotkey") {
		t.Fatalf("sample JSON grew new fields: %s", text)
	}
	out, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip\n got %#v\nwant %#v", out, in)
	}
	if out.Steps[1].Clicks != 0 {
		t.Fatalf("clicks 0 was rewritten to %d", out.Steps[1].Clicks)
	}

	path := filepath.Join(t.TempDir(), "script.json")
	if err := in.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, loaded) {
		t.Fatalf("save/load\n got %#v\nwant %#v", loaded, in)
	}
}

func TestUnmarshalBadJSON(t *testing.T) {
	if _, err := Unmarshal([]byte("{")); err == nil {
		t.Fatal("expected error")
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestRichJSONRoundTrip(t *testing.T) {
	in := &Script{
		Version:           1,
		Name:              "進階",
		Repeat:            2,
		DefaultIntervalMs: 10,
		EmergencyHotkey:   "F12",
		Steps: []Step{
			{X: 1, Y: 2, Button: ButtonLeft, Clicks: 1, Repeat: 3, IntervalMs: 5},
			{
				Button: ButtonRight, Clicks: 1, IntervalMs: 8,
				Target: &Target{Kind: TargetCircle, CX: 9, CY: 8, R: 3},
			},
			{
				Button: ButtonMiddle,
				Target: &Target{
					Kind: TargetImage, Template: "b.png", Threshold: 0.9,
					Region: &Region{X: 1, Y: 2, W: 3, H: 4}, OffsetX: -2, OffsetY: 5,
					OnNotFound: OnNotFoundStop, TimeoutMs: 10, PollMs: 20,
				},
			},
		},
	}
	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	out, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("got %#v\nwant %#v", out, in)
	}
	if err := out.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestOldScriptJSON(t *testing.T) {
	const raw = `{
  "version": 1,
  "name": "示範腳本",
  "steps": [
    {"x": 100, "y": 200, "button": "left", "clicks": 1, "interval_ms": 50, "hold_ms": 0},
    {"x": 300, "y": 400, "button": "right", "clicks": 2, "interval_ms": 0, "hold_ms": 30}
  ],
  "repeat": 1,
  "default_interval_ms": 100,
  "loop_interval_ms": 500,
  "start_delay_ms": 200
}`
	s, err := Unmarshal([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if s.Steps[0].Repeat != 0 || s.Steps[0].Target != nil || s.Steps[0].EffectiveRepeat() != 1 {
		t.Fatalf("%+v", s.Steps[0])
	}
	if s.EmergencyHotkey != "" {
		t.Fatal(s.EmergencyHotkey)
	}
}

func TestEffectiveClicksAndInterval(t *testing.T) {
	s := Step{Clicks: 0, IntervalMs: 0}
	if s.EffectiveClicks() != 1 {
		t.Fatal(s.EffectiveClicks())
	}
	if s.EffectiveIntervalMs(40) != 40 {
		t.Fatal(s.EffectiveIntervalMs(40))
	}
	s.Clicks = 3
	s.IntervalMs = 15
	if s.EffectiveClicks() != 3 || s.EffectiveIntervalMs(40) != 15 {
		t.Fatalf("%d %d", s.EffectiveClicks(), s.EffectiveIntervalMs(40))
	}
	if (Step{}).EffectiveRepeat() != 1 || (Step{Repeat: 1}).EffectiveRepeat() != 1 || (Step{Repeat: 4}).EffectiveRepeat() != 4 {
		t.Fatal("repeat")
	}
	if (Target{}).EffectiveThreshold() != DefaultImageThreshold || (Target{Threshold: 0.5}).EffectiveThreshold() != 0.5 {
		t.Fatal("threshold")
	}
	if (Target{}).NotFoundOrSkip() != OnNotFoundSkip || (Target{Kind: ""}).KindOrPoint() != TargetPoint {
		t.Fatal("defaults")
	}
}

func cloneScript(s *Script) *Script {
	cp := *s
	cp.Steps = append([]Step(nil), s.Steps...)
	return &cp
}
