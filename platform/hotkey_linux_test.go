//go:build linux

package platform

import (
	"testing"

	"github.com/click-spirit/click-spirit/core"
)

func TestX11ModifierMask(t *testing.T) {
	c, err := core.ParseHotkey("Ctrl+Shift+F12")
	if err != nil {
		t.Fatal(err)
	}
	if got := x11Mods(c); got != x11Control|x11Shift {
		t.Fatalf("mask %d, want %d", got, x11Control|x11Shift)
	}
	c, err = core.ParseHotkey("Alt+Super+Esc")
	if err != nil {
		t.Fatal(err)
	}
	if got := x11Mods(c); got != x11Mod1|x11Mod4 {
		t.Fatalf("mask %d, want %d", got, x11Mod1|x11Mod4)
	}
	c, err = core.ParseHotkey("F12")
	if err != nil {
		t.Fatal(err)
	}
	if x11Mods(c) != 0 {
		t.Fatal(x11Mods(c))
	}
}
