//go:build windows

package platform

import (
	"testing"

	"github.com/click-spirit/click-spirit/core"
)

func TestWindowsHotkeyMapping(t *testing.T) {
	c, err := core.ParseHotkey("Ctrl+Shift+F12")
	if err != nil {
		t.Fatal(err)
	}
	if got := winMods(c); got != modControl|modShift|modNoRepeat {
		t.Fatalf("mods %#x", got)
	}
	vk, err := vkOf("f12")
	if err != nil || vk != 0x7B {
		t.Fatalf("f12 %#x %v", vk, err)
	}
	vk, err = vkOf("escape")
	if err != nil || vk != 0x1B {
		t.Fatalf("escape %#x %v", vk, err)
	}
	vk, err = vkOf("a")
	if err != nil || vk != 0x41 {
		t.Fatalf("a %#x %v", vk, err)
	}
	vk, err = vkOf("5")
	if err != nil || vk != 0x35 {
		t.Fatalf("5 %#x %v", vk, err)
	}
}
