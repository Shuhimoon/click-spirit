package engine

import (
	"fmt"

	"github.com/click-spirit/click-spirit/core"
)

func sprintfStep(step core.Step, rep int) string {
	base := fmt.Sprintf("點擊 %s (%d,%d) x%d", step.Button, step.X, step.Y, step.EffectiveClicks())
	if step.EffectiveRepeat() > 1 {
		return fmt.Sprintf("%s #%d", base, rep+1)
	}
	return base
}

func sprintfLoop(loop int) string {
	return fmt.Sprintf("迴圈 %d 完成", loop+1)
}

// ClickLabel is the overlay caption for a click event, e.g. "步驟 2 · 第 3/10 次".
// Step numbers are one-based. A step that runs once omits the repetition.
func ClickLabel(ev core.Event) string {
	n := ev.Repeats
	if n < 1 {
		n = ev.Step.EffectiveRepeat()
	}
	if n <= 1 {
		return fmt.Sprintf("步驟 %d", ev.StepIndex+1)
	}
	return fmt.Sprintf("步驟 %d · 第 %d/%d 次", ev.StepIndex+1, ev.Repetition+1, n)
}

// CountdownLabel is the start-delay caption, e.g. "3 秒後開始".
func CountdownLabel(sec int) string {
	return fmt.Sprintf("%d 秒後開始", sec)
}
