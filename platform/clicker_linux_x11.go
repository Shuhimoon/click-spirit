//go:build linux && cgo && !nox11

package platform

/*
#cgo pkg-config: x11 xtst
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/extensions/XTest.h>

static void csInitThreads(void) {
	XInitThreads();
}

// Xvfb on a busy machine resets some new connections (ECONNRESET / EAGAIN).
// A short retry succeeds. A display that is actually down still returns NULL.
static Display *csOpen(void) {
	int i;
	for (i = 0; i < 20; i++) {
		Display *d = XOpenDisplay(NULL);
		if (d != NULL || i == 19) {
			return d;
		}
		usleep(1000);
	}
	return NULL;
}

static void csClose(Display *dpy) {
	if (dpy != NULL) {
		XCloseDisplay(dpy);
	}
}

static int csXTest(Display *dpy) {
	int eventBase = 0, errorBase = 0, major = 0, minor = 0;
	if (dpy == NULL) {
		return 0;
	}
	return XTestQueryExtension(dpy, &eventBase, &errorBase, &major, &minor);
}

// screen -1 is the screen the pointer is on. Returns 1 on success.
static int csMove(Display *dpy, int x, int y) {
	if (dpy == NULL) {
		return -1;
	}
	int rc = XTestFakeMotionEvent(dpy, -1, x, y, 0);
	XFlush(dpy);
	XSync(dpy, False);
	return rc;
}

// button is 1 left, 2 middle, 3 right. isPress is 1 for down, 0 for up.
static int csButton(Display *dpy, unsigned int button, int isPress) {
	if (dpy == NULL) {
		return -1;
	}
	int rc = XTestFakeButtonEvent(dpy, button, isPress ? True : False, 0);
	XFlush(dpy);
	XSync(dpy, False);
	return rc;
}

static int csCursor(Display *dpy, int *x, int *y) {
	if (dpy == NULL || x == NULL || y == NULL) {
		return -1;
	}
	Window root = XDefaultRootWindow(dpy);
	Window rootRet = 0, childRet = 0;
	int rx = 0, ry = 0, wx = 0, wy = 0;
	unsigned int mask = 0;
	if (!XQueryPointer(dpy, root, &rootRet, &childRet, &rx, &ry, &wx, &wy, &mask)) {
		return -1;
	}
	*x = rx;
	*y = ry;
	return 0;
}

typedef struct {
	int width;
	int height;
	int bits_per_pixel;
	int bytes_per_line;
	int byte_order;
	unsigned long red_mask;
	unsigned long green_mask;
	unsigned long blue_mask;
} csImageInfo;

// csCapture copies a ZPixmap of the root window. Caller frees *out with free.
static int csCapture(Display *dpy, csImageInfo *info, unsigned char **out, int *outLen) {
	if (out != NULL) {
		*out = NULL;
	}
	if (outLen != NULL) {
		*outLen = 0;
	}
	if (dpy == NULL || info == NULL || out == NULL || outLen == NULL) {
		return -1;
	}
	int screen = XDefaultScreen(dpy);
	Window root = XRootWindow(dpy, screen);
	int width = XDisplayWidth(dpy, screen);
	int height = XDisplayHeight(dpy, screen);
	if (width <= 0 || height <= 0) {
		return -2;
	}
	XImage *img = XGetImage(dpy, root, 0, 0, (unsigned int)width, (unsigned int)height, AllPlanes, ZPixmap);
	if (img == NULL || img->data == NULL) {
		if (img != NULL) {
			XDestroyImage(img);
		}
		return -3;
	}
	int n = img->bytes_per_line * img->height;
	if (n <= 0) {
		XDestroyImage(img);
		return -3;
	}
	unsigned char *buf = (unsigned char *)malloc((size_t)n);
	if (buf == NULL) {
		XDestroyImage(img);
		return -4;
	}
	memcpy(buf, img->data, (size_t)n);
	info->width = img->width;
	info->height = img->height;
	info->bits_per_pixel = img->bits_per_pixel;
	info->bytes_per_line = img->bytes_per_line;
	info->byte_order = img->byte_order;
	info->red_mask = img->red_mask;
	info->green_mask = img->green_mask;
	info->blue_mask = img->blue_mask;
	XDestroyImage(img);
	*out = buf;
	*outLen = n;
	return 0;
}
*/
import "C"
import (
	"errors"
	"fmt"
	"image"
	"math/bits"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/click-spirit/click-spirit/core"
)

func init() {
	// XInitThreads must run before any other Xlib call in the process.
	// The Flutter shell loads this library before GTK starts so the lock
	// exists for both. Each clicker still serializes its own Display.
	C.csInitThreads()
}

func x11BuiltIn() bool { return true }

type x11Clicker struct {
	mu  sync.Mutex
	dpy *C.Display
	x   int
	y   int
}

var (
	_ core.Clicker        = (*x11Clicker)(nil)
	_ core.CursorLocator  = (*x11Clicker)(nil)
	_ core.ScreenCapturer = (*x11Clicker)(nil)
)

// newX11 opens DISPLAY and checks that the XTest extension is present.
// All later Xlib calls on this Display take x11Clicker.mu.
func newX11() (core.Clicker, error) {
	dpy := C.csOpen()
	if dpy == nil {
		return nil, fmt.Errorf("XOpenDisplay failed (DISPLAY=%q)", os.Getenv("DISPLAY"))
	}
	if C.csXTest(dpy) == 0 {
		C.csClose(dpy)
		return nil, errors.New("XTest extension is not available")
	}
	return &x11Clicker{dpy: dpy}, nil
}

// Close releases the X display. Later methods return an error.
func (c *x11Clicker) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dpy != nil {
		C.csClose(c.dpy)
		c.dpy = nil
	}
	return nil
}

func (c *x11Clicker) MoveTo(x, y int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dpy == nil {
		return errors.New("X11 display is closed")
	}
	if C.csMove(c.dpy, C.int(x), C.int(y)) <= 0 {
		return errors.New("XTestFakeMotionEvent failed")
	}
	c.x, c.y = x, y
	return nil
}

func (c *x11Clicker) Click(b core.Button, count, holdMs int) error {
	if count <= 0 {
		count = 1
	}
	button, err := x11Button(b)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dpy == nil {
		return errors.New("X11 display is closed")
	}
	for i := 0; i < count; i++ {
		if C.csButton(c.dpy, C.uint(button), 1) <= 0 {
			return errors.New("XTestFakeButtonEvent press failed")
		}
		if holdMs > 0 {
			time.Sleep(time.Duration(holdMs) * time.Millisecond)
		}
		if C.csButton(c.dpy, C.uint(button), 0) <= 0 {
			return errors.New("XTestFakeButtonEvent release failed")
		}
	}
	return nil
}

func (c *x11Clicker) CursorPos() (int, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dpy == nil {
		return 0, 0, errors.New("X11 display is closed")
	}
	var x, y C.int
	if C.csCursor(c.dpy, &x, &y) != 0 {
		return 0, 0, errors.New("XQueryPointer failed")
	}
	return int(x), int(y), nil
}

// Capture grabs the root window as a ZPixmap and converts it with the
// server's channel masks.
func (c *x11Clicker) Capture() (image.Image, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dpy == nil {
		return nil, errors.New("X11 display is closed")
	}
	var info C.csImageInfo
	var buf *C.uchar
	var n C.int
	rc := C.csCapture(c.dpy, &info, &buf, &n)
	if buf != nil {
		defer C.free(unsafe.Pointer(buf))
	}
	if rc != 0 {
		return nil, fmt.Errorf("XGetImage failed (%d)", int(rc))
	}
	raw := C.GoBytes(unsafe.Pointer(buf), n)
	return rgbaFromZPixmap(int(info.width), int(info.height), int(info.bits_per_pixel), int(info.bytes_per_line), int(info.byte_order), uint32(info.red_mask), uint32(info.green_mask), uint32(info.blue_mask), raw)
}

func x11Button(b core.Button) (uint, error) {
	switch b {
	case core.ButtonLeft:
		return 1, nil
	case core.ButtonMiddle:
		return 2, nil
	case core.ButtonRight:
		return 3, nil
	default:
		return 0, fmt.Errorf("invalid button %q", b)
	}
}

// rgbaFromZPixmap converts an X ZPixmap into an RGBA image using the red,
// green, and blue masks. byteOrder is X11's LSBFirst (0) or MSBFirst (1).
// Alpha is opaque because a typical TrueColor root window has no alpha mask.
func rgbaFromZPixmap(width, height, bpp, bpl, byteOrder int, red, green, blue uint32, raw []byte) (*image.RGBA, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("empty capture")
	}
	if bpp != 16 && bpp != 24 && bpp != 32 {
		return nil, fmt.Errorf("unsupported bits per pixel %d", bpp)
	}
	pixBytes := bpp / 8
	if bpl < width*pixBytes {
		return nil, fmt.Errorf("bytes_per_line %d shorter than width", bpl)
	}
	need := bpl * height
	if len(raw) < need || need < 0 {
		return nil, fmt.Errorf("capture buffer %d shorter than %d", len(raw), need)
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		row := raw[y*bpl:]
		for x := 0; x < width; x++ {
			p := readZPixel(row[x*pixBytes:], bpp, byteOrder)
			off := (y*width + x) * 4
			dst.Pix[off] = maskTo8(p, red)
			dst.Pix[off+1] = maskTo8(p, green)
			dst.Pix[off+2] = maskTo8(p, blue)
			dst.Pix[off+3] = 255
		}
	}
	return dst, nil
}

func readZPixel(b []byte, bpp, byteOrder int) uint32 {
	switch bpp {
	case 32:
		if byteOrder == 0 {
			return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
		}
		return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	case 24:
		if byteOrder == 0 {
			return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
		}
		return uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
	default:
		if byteOrder == 0 {
			return uint32(b[0]) | uint32(b[1])<<8
		}
		return uint32(b[0])<<8 | uint32(b[1])
	}
}

func maskTo8(pixel, mask uint32) uint8 {
	if mask == 0 {
		return 0
	}
	shift := bits.TrailingZeros32(mask)
	width := bits.OnesCount32(mask)
	val := (pixel & mask) >> shift
	if width >= 8 {
		return uint8(val >> (width - 8))
	}
	return uint8((val * 255) / ((1 << width) - 1))
}
