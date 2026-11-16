//go:build linux

package platform

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

// ffmpegVideo records the X11 screen with an `ffmpeg -f x11grab`
// subprocess (錄影轉腳本). Stop sends "q" so the mp4 is finalised.
type ffmpegVideo struct {
	ffmpeg string
	size   func() (int, int)

	mu    sync.Mutex
	cmd   *exec.Cmd
	stdin io.WriteCloser
	done  chan error
}

// NewVideoRecorder returns the screen video backend: ffmpeg x11grab on X11
// when ffmpeg is installed. Otherwise ErrUnsupported (clicks are still
// recorded, without video).
func NewVideoRecorder() (core.VideoRecorder, error) {
	if Name() != "x11" || os.Getenv("DISPLAY") == "" {
		return nil, fmt.Errorf("screen video needs X11: %w", ErrUnsupported)
	}
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found: %w", ErrUnsupported)
	}
	return &ffmpegVideo{ffmpeg: bin, size: screenSize}, nil
}

func ffmpegArgs(display string, w, h int, path string) []string {
	return []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "x11grab", "-draw_mouse", "1", "-framerate", "15",
		"-video_size", strconv.Itoa(w) + "x" + strconv.Itoa(h),
		"-i", display + "+0,0",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		path,
	}
}

func (v *ffmpegVideo) Start(path string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cmd != nil {
		return errors.New("already recording video")
	}
	w, h := v.size()
	if w <= 0 || h <= 0 {
		return errors.New("cannot read the screen size")
	}
	// libx264 needs even sizes.
	w, h = w&^1, h&^1
	cmd := exec.Command(v.ffmpeg, ffmpegArgs(os.Getenv("DISPLAY"), w, h, path)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	var stderr limitedBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	// Fail fast when ffmpeg exits at once (bad display, missing codec).
	select {
	case err := <-done:
		return fmt.Errorf("ffmpeg exited: %v %s", err, stderr.String())
	case <-time.After(300 * time.Millisecond):
	}
	v.cmd, v.stdin, v.done = cmd, stdin, done
	return nil
}

func (v *ffmpegVideo) Stop() error {
	v.mu.Lock()
	cmd, stdin, done := v.cmd, v.stdin, v.done
	v.cmd, v.stdin, v.done = nil, nil, nil
	v.mu.Unlock()
	if cmd == nil {
		return errors.New("not recording video")
	}
	_, _ = io.WriteString(stdin, "q\n")
	_ = stdin.Close()
	select {
	case <-done:
		return nil
	case <-time.After(8 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return errors.New("ffmpeg did not stop in time; the video may be incomplete")
	}
}

// VideoFrame writes the frame at atMs of video as a PNG thumbnail (used by
// the review screen). It needs ffmpeg.
func VideoFrame(video string, atMs int64, out string, width int) error {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found: %w", ErrUnsupported)
	}
	if width <= 0 {
		width = 320
	}
	ss := strconv.FormatFloat(float64(atMs)/1000, 'f', 3, 64)
	cmd := exec.Command(bin, "-hide_banner", "-loglevel", "error", "-y", "-ss", ss, "-i", video,
		"-frames:v", "1", "-vf", "scale="+strconv.Itoa(width)+":-2", out)
	var stderr limitedBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg frame: %v %s", err, stderr.String())
	}
	return nil
}

type limitedBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.buf) < 4096 {
		b.buf = append(b.buf, p...)
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// ScreenSize is the recorded area in screen pixels (the review screen uses
// it to place click dots on thumbnails).
func (v *ffmpegVideo) ScreenSize() (int, int) { return v.size() }
