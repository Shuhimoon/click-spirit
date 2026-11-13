package core

// ClickOverlay draws a short-lived marker (a ripple and a label) at the screen
// position of a click that was just performed. Implementations must not
// intercept pointer or touch input and must return quickly: ShowClick is
// called on the runner goroutine between clicks.
type ClickOverlay interface {
	ShowClick(x, y int, label string)
}
