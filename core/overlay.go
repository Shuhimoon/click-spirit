package core

// ClickOverlay draws a short-lived marker (a ripple and a label) at the screen
// position of a click that was just performed. Implementations must not
// intercept pointer or touch input and must return quickly: ShowClick is
// called on the runner goroutine between clicks.
type ClickOverlay interface {
	ShowClick(x, y int, label string)
}

// CountdownOverlay is optionally implemented by a ClickOverlay to show the
// start-delay countdown ("3 秒後開始") on top of everything. ShowCountdown(0)
// hides it. It must not intercept input and must return quickly.
type CountdownOverlay interface {
	ShowCountdown(seconds int)
}
