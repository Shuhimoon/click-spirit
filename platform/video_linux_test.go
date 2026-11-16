//go:build linux && cgo && !nox11

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFfmpegArgs(t *testing.T) {
	args := ffmpegArgs(":99", 1600, 1000, "/tmp/a.mp4")
	joined := ""
	for _, a := range args {
		joined += a + " "
	}
	for _, want := range []string{"-f x11grab", "-video_size 1600x1000", "-i :99+0,0", "/tmp/a.mp4"} {
		if !contains(joined, want) {
			t.Fatalf("args %q missing %q", joined, want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestX11ScreenVideo records 1.5 s with ffmpeg x11grab under xvfb-run.
func TestX11ScreenVideo(t *testing.T) {
	if os.Getenv("CLICK_SPIRIT_X11_TEST") != "1" {
		t.Skip("set CLICK_SPIRIT_X11_TEST=1 under xvfb-run")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	disp := os.Getenv("DISPLAY")
	if disp == "" || forbiddenSharedDisplay(disp) {
		t.Fatalf("refusing display %q; run under xvfb-run", disp)
	}
	t.Setenv("CLICK_SPIRIT_BACKEND", "x11")
	v, err := NewVideoRecorder()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "rec.mp4")
	if err := v.Start(path); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	if err := v.Stop(); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() < 1000 {
		t.Fatalf("video %v size %v", err, st)
	}
	png := filepath.Join(dir, "f.png")
	if err := VideoFrame(path, 500, png, 160); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(png); err != nil || st.Size() < 100 {
		t.Fatalf("thumbnail %v", err)
	}
}
