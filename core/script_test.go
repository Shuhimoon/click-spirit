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
}

func cloneScript(s *Script) *Script {
	cp := *s
	cp.Steps = append([]Step(nil), s.Steps...)
	return &cp
}
