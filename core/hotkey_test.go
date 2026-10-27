package core

import "testing"

func TestParseHotkey(t *testing.T) {
	cases := []struct {
		in    string
		key   string
		ctrl  bool
		shift bool
		alt   bool
		super bool
	}{
		{in: "F12", key: "f12"},
		{in: "Ctrl+Shift+Esc", key: "escape", ctrl: true, shift: true},
		{in: "ctrl+alt+a", key: "a", ctrl: true, alt: true},
		{in: "Win+F1", key: "f1", super: true},
		{in: "shift+ctrl+escape", key: "escape", ctrl: true, shift: true},
		{in: "Command+Option+Space", key: "space", alt: true, super: true},
		{in: "f24", key: "f24"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			c, err := ParseHotkey(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if c.Key != tc.key || c.Ctrl != tc.ctrl || c.Shift != tc.shift || c.Alt != tc.alt || c.Super != tc.super {
				t.Fatalf("%+v", c)
			}
		})
	}
	for _, bad := range []string{"", "Ctrl+", "Ctrl++Esc", "Foo+F1", "F12+F11", "not-a-key", "F0", "F25"} {
		if _, err := ParseHotkey(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}
