//go:build linux && cgo && !nox11

package platform

import (
	"os"
	"testing"
	"time"
)

func TestX11Bubble(t *testing.T) {
	if os.Getenv("CLICK_SPIRIT_X11_TEST") != "1" {
		t.Skip("set CLICK_SPIRIT_X11_TEST=1 under xvfb-run")
	}
	disp := os.Getenv("DISPLAY")
	if disp == "" || forbiddenSharedDisplay(disp) {
		t.Fatalf("refusing display %q; run under xvfb-run", disp)
	}
	t.Setenv("CLICK_SPIRIT_BACKEND", "x11")
	bb, err := NewBubble()
	if err != nil {
		t.Fatal(err)
	}
	b := bb.(*x11Bubble)
	defer b.Close()
	actions := make(chan string, 4)
	b.SetOnAction(func(a string) { actions <- a })
	b.SetVisible(true)
	time.Sleep(200 * time.Millisecond)

	r, ok := b.Rect()
	if !ok || r.W != 56 || r.H != 56 {
		t.Fatalf("collapsed rect %+v visible=%v", r, ok)
	}
	tap := func(x, y int) {
		pickFakeMotion(x, y)
		time.Sleep(40 * time.Millisecond)
		pickFakeButton(1, true)
		pickFakeButton(1, false)
		time.Sleep(200 * time.Millisecond)
	}
	cx, cy := r.X+28, r.Y+28
	tap(cx, cy)
	e, _ := b.Rect()
	if e.W <= 56 || !e.Contains(cx, cy) {
		t.Fatalf("expanded rect %+v does not hold the bubble", e)
	}
	// Buttons open to the left near the right edge: 錄製, 開始, 停止.
	startX := e.X + 6 + 84 + 42
	tap(startX, cy)
	select {
	case a := <-actions:
		if a != "start" {
			t.Fatalf("action %q want start", a)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no action from 開始腳本")
	}
	if c, _ := b.Rect(); c.W != 56 {
		t.Fatalf("not collapsed after an action: %+v", c)
	}

	// Drag moves it and is not a tap.
	pickFakeMotion(cx, cy)
	pickFakeButton(1, true)
	for i := 1; i <= 10; i++ {
		pickFakeMotion(cx-20*i, cy-10*i)
		time.Sleep(15 * time.Millisecond)
	}
	pickFakeButton(1, false)
	time.Sleep(200 * time.Millisecond)
	m, _ := b.Rect()
	if m.X != r.X-200 || m.Y != r.Y-100 || m.W != 56 {
		t.Fatalf("after drag %+v (from %+v)", m, r)
	}
	select {
	case a := <-actions:
		t.Fatalf("drag produced action %q", a)
	default:
	}
	b.SetState(true, false)
	b.SetVisible(false)
	time.Sleep(150 * time.Millisecond)
	if _, vis := b.Rect(); vis {
		t.Fatal("still visible")
	}
}
