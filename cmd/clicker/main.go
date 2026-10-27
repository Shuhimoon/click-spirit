package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"image"
	"image/png"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/core/imagematch"
	"github.com/click-spirit/click-spirit/platform"
)

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	if len(args) < 2 {
		usage()
		return 2
	}
	switch args[1] {
	case "sample":
		return cmdSample(args[2:])
	case "validate":
		return cmdValidate(args[2:])
	case "run":
		return cmdRun(args[2:])
	case "pos":
		return cmdPos()
	case "match":
		return cmdMatch(args[2:])
	case "-h", "-help", "--help", "help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "未知指令 %q\n", args[1])
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `點擊精靈 clicker

用法:
  clicker sample -o file.json
  clicker validate file.json
  clicker run [-dry-run] file.json
  clicker pos
  clicker match [-threshold 0.9] template.png screenshot.png

run 會在收到 Ctrl+C 時停止。腳本的 emergency_hotkey 會在非 dry-run 時武裝。
-dry-run 只印出動作，不移動游標。
pos 印出游標座標；此平台若不支援會結束並回傳錯誤。
match 在截圖裡找模板，印出 JSON。找到且沒有錯誤時結束碼為 0。
`)
}

func cmdSample(args []string) int {
	fs := flag.NewFlagSet("sample", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("o", "", "輸出 JSON 路徑")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *out == "" {
		fmt.Fprintln(os.Stderr, "sample: 需要 -o 路徑")
		return 2
	}
	if err := core.NewSample().Save(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(*out)
	return 0
}

func cmdValidate(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "validate: 需要一個 JSON 檔")
		return 2
	}
	s, err := core.Load(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := s.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("ok")
	return 0
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dry := fs.Bool("dry-run", false, "只記錄點擊，不控制游標")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "run: 需要一個 JSON 檔")
		return 2
	}
	script, err := core.Load(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := script.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, "驗證失敗:", err)
		return 1
	}

	var clicker core.Clicker
	if *dry {
		clicker = dryClicker{}
		fmt.Println("dry-run")
	} else {
		clicker, err = platform.New()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("backend", platform.Name())
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := &core.Runner{
		Clicker: clicker,
		OnEvent: printEvent,
	}
	if cap, ok := clicker.(core.ScreenCapturer); ok {
		runner.Capturer = cap
	}
	if !*dry && script.EmergencyHotkey != "" {
		src, herr := platform.NewHotkey(script.EmergencyHotkey)
		if herr != nil {
			fmt.Fprintln(os.Stderr, herr)
			return 1
		}
		if herr = src.Start(func() { runner.Stop() }); herr != nil {
			src.Stop()
			fmt.Fprintln(os.Stderr, herr)
			return 1
		}
		defer src.Stop()
	}
	err = runner.Run(ctx, script)
	if err != nil {
		if errors.Is(err, core.ErrStopped) {
			fmt.Println("已停止")
			return 0
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("完成")
	return 0
}

func cmdPos() int {
	c, err := platform.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	loc, ok := c.(core.CursorLocator)
	if !ok {
		fmt.Fprintln(os.Stderr, "此平台不支援讀取游標座標")
		return 1
	}
	x, y, err := loc.CursorPos()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("%d %d\n", x, y)
	return 0
}

func printEvent(ev core.Event) {
	fmt.Printf("event kind=%s loop=%d step=%d rep=%d x=%d y=%d button=%s\n",
		ev.Kind, ev.Loop, ev.StepIndex, ev.Repetition, ev.Step.X, ev.Step.Y, ev.Step.Button)
	if ev.Err != nil {
		fmt.Printf("event error %s\n", ev.Err.Error())
	}
}

type dryClicker struct{}

func (dryClicker) MoveTo(x, y int) error {
	fmt.Printf("move %d %d\n", x, y)
	return nil
}

func (dryClicker) Click(b core.Button, count, holdMs int) error {
	fmt.Printf("click %s x%d hold %dms\n", b, count, holdMs)
	return nil
}

func cmdMatch(args []string) int {
	fs := flag.NewFlagSet("match", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	th := fs.Float64("threshold", 0.9, "分數門檻，小於等於 0 時用 0.9")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "match: 需要 template.png 與 screenshot.png")
		return 2
	}
	templ, err := loadPNG(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	screen, err := loadPNG(fs.Arg(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	hit := imagematch.MatchImages(screen, templ, *th)
	fmt.Println(hit.JSON())
	if hit.Error != "" || !hit.Found {
		return 1
	}
	return 0
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}
