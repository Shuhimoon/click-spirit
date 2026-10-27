package core

import (
	"fmt"
	"strconv"
	"strings"
)

// DefaultHotkey is the suggested emergency-stop chord.
const DefaultHotkey = "F12"

// HotkeySource delivers one global key chord.
// Start returns after the hook is armed. The callback runs on the source's
// own goroutine and must not call Stop on the same source.
// Stop releases the hook and waits until the event loop has exited.
// Stop is safe to call more than once, including before a successful Start.
type HotkeySource interface {
	Start(fn func()) error
	Stop()
}

// Chord is a parsed hotkey. Key is a canonical name such as "f12" or "escape".
type Chord struct {
	Key   string
	Ctrl  bool
	Shift bool
	Alt   bool
	Super bool
}

// ParseHotkey parses chords such as "F12" and "Ctrl+Shift+Esc".
// Names are case-insensitive. Modifier order does not matter.
func ParseHotkey(name string) (Chord, error) {
	raw := strings.TrimSpace(name)
	if raw == "" {
		return Chord{}, fmt.Errorf("empty hotkey")
	}
	parts := strings.Split(raw, "+")
	var c Chord
	var key string
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			return Chord{}, fmt.Errorf("hotkey %q has an empty part", name)
		}
		switch part {
		case "ctrl", "control":
			c.Ctrl = true
		case "shift":
			c.Shift = true
		case "alt", "option":
			c.Alt = true
		case "super", "win", "meta", "cmd", "command":
			c.Super = true
		default:
			if key != "" {
				return Chord{}, fmt.Errorf("hotkey %q has more than one key", name)
			}
			canon, err := canonicalKey(part)
			if err != nil {
				return Chord{}, fmt.Errorf("hotkey %q: %w", name, err)
			}
			key = canon
		}
	}
	if key == "" {
		return Chord{}, fmt.Errorf("hotkey %q has no key", name)
	}
	c.Key = key
	return c, nil
}

func canonicalKey(part string) (string, error) {
	switch part {
	case "esc", "escape":
		return "escape", nil
	case "enter", "return":
		return "return", nil
	case "pgup", "pageup", "page_up":
		return "pageup", nil
	case "pgdn", "pagedown", "page_down":
		return "pagedown", nil
	case "bs", "backspace":
		return "backspace", nil
	case "del", "delete":
		return "delete", nil
	case "ins", "insert":
		return "insert", nil
	case "space", "spacebar":
		return "space", nil
	case "tab", "home", "end", "left", "right", "up", "down":
		return part, nil
	}
	if len(part) >= 2 && part[0] == 'f' {
		n, err := strconv.Atoi(part[1:])
		if err == nil && n >= 1 && n <= 24 {
			return "f" + strconv.Itoa(n), nil
		}
	}
	if len(part) == 1 {
		c := part[0]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			return part, nil
		}
	}
	return "", fmt.Errorf("unknown key %q", part)
}
