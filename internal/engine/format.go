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
