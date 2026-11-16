package core

// Bubble actions reported by the floating bubble (小浮窗).
const (
	BubbleRecord = "record" // start or stop recording click positions
	BubbleStart  = "start"  // run the default script
	BubbleStop   = "stop"   // stop the running script
)

// Bubble is a small always-on-top, draggable floating button (like iPhone
// AssistiveTouch). Tapping it expands three buttons that report an action.
// Rect is the area it covers right now (expanded or not) in clicker
// coordinates; scripts never click there and recordings drop presses there.
type Bubble interface {
	SetVisible(on bool)
	Rect() (Rect, bool)
	// SetState updates the button labels (錄製/停止錄製, running highlight).
	SetState(recording, running bool)
	SetOnAction(fn func(action string))
}

// VideoRecorder records the screen to a video file until Stop.
type VideoRecorder interface {
	Start(path string) error
	Stop() error
}
