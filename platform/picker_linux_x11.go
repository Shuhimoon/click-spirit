//go:build linux && cgo && !nox11

package platform

/*
#cgo pkg-config: x11 xext xft xtst
#cgo LDFLAGS: -lm
#include <math.h>
#include <poll.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>
#include <X11/cursorfont.h>
#include <X11/Xft/Xft.h>
#include <X11/extensions/XTest.h>

// The picker grabs a screenshot of the root window, covers the whole root
// with an override-redirect window that shows the frozen screenshot 1:1,
// and grabs pointer and keyboard. Layer pixel (x, y) is root pixel
// (origin + x, origin + y), which is exactly what XTest clicks.

typedef struct {
	Display *dpy;
	int screen;
	Window root, win;
	Visual *visual;
	Colormap cmap;
	int depth, w, h;
	XImage *img;
	Pixmap shot, back, stipple;
	GC gc;
	XftDraw *xd;
	XftFont *font;
	XftColor white;
	unsigned long blue, ink, light, dark;
	int ox, oy;
	int circle;
	int mx, my, pressed, px, py;
} csPicker;

static unsigned long pkRGB(csPicker *p, int r, int g, int b) {
	XColor c;
	memset(&c, 0, sizeof c);
	c.red = r << 8; c.green = g << 8; c.blue = b << 8;
	c.flags = DoRed | DoGreen | DoBlue;
	if (!XAllocColor(p->dpy, p->cmap, &c)) return BlackPixel(p->dpy, p->screen);
	return c.pixel;
}

static int pkTextWidth(csPicker *p, const char *s) {
	XGlyphInfo g;
	if (p->font == NULL) return 0;
	XftTextExtentsUtf8(p->dpy, p->font, (const FcChar8 *)s, (int)strlen(s), &g);
	return g.xOff;
}

static void pkPill(csPicker *p, int x, int y, const char *s) {
	int tw = pkTextWidth(p, s);
	int w = tw + 20, h = 26;
	if (x + w > p->w - 4) x = p->w - 4 - w;
	if (y + h > p->h - 4) y = p->h - 4 - h;
	if (x < 4) x = 4;
	if (y < 4) y = 4;
	XSetForeground(p->dpy, p->gc, p->ink);
	XFillRectangle(p->dpy, p->back, p->gc, x + h / 2, y, w - h, h);
	XFillArc(p->dpy, p->back, p->gc, x, y, h, h, 90 * 64, 180 * 64);
	XFillArc(p->dpy, p->back, p->gc, x + w - h, y, h, h, 270 * 64, 180 * 64);
	if (p->font != NULL) {
		int ty = y + (h + p->font->ascent - p->font->descent) / 2;
		XftDrawStringUtf8(p->xd, &p->white, p->font, x + 10, ty, (const FcChar8 *)s, (int)strlen(s));
	}
}

static void pkLine(csPicker *p, int x1, int y1, int x2, int y2) {
	XSetLineAttributes(p->dpy, p->gc, 3, LineSolid, CapButt, JoinMiter);
	XSetForeground(p->dpy, p->gc, p->dark);
	XDrawLine(p->dpy, p->back, p->gc, x1, y1, x2, y2);
	XSetLineAttributes(p->dpy, p->gc, 1, LineSolid, CapButt, JoinMiter);
	XSetForeground(p->dpy, p->gc, p->light);
	XDrawLine(p->dpy, p->back, p->gc, x1, y1, x2, y2);
}

// 15x15 pixels around the pointer, 8x zoomed, centre pixel outlined.
static void pkLoupe(csPicker *p, int lx, int ly) {
	const int n = 15, z = 8, half = 7, size = n * z;
	XSetForeground(p->dpy, p->gc, p->dark);
	XFillRectangle(p->dpy, p->back, p->gc, lx - 2, ly - 2, size + 4, size + 4);
	for (int j = 0; j < n; j++) {
		for (int i = 0; i < n; i++) {
			int sx = p->mx - half + i, sy = p->my - half + j;
			unsigned long px = p->dark;
			if (sx >= 0 && sy >= 0 && sx < p->img->width && sy < p->img->height) px = XGetPixel(p->img, sx, sy);
			XSetForeground(p->dpy, p->gc, px);
			XFillRectangle(p->dpy, p->back, p->gc, lx + i * z, ly + j * z, z, z);
		}
	}
	XSetLineAttributes(p->dpy, p->gc, 1, LineSolid, CapButt, JoinMiter);
	XSetForeground(p->dpy, p->gc, p->light);
	XDrawRectangle(p->dpy, p->back, p->gc, lx + half * z - 1, ly + half * z - 1, z + 1, z + 1);
	XSetForeground(p->dpy, p->gc, p->blue);
	XDrawRectangle(p->dpy, p->back, p->gc, lx - 1, ly - 1, size + 1, size + 1);
}

static void pkRedraw(csPicker *p) {
	Display *d = p->dpy;
	char buf[128];
	XCopyArea(d, p->shot, p->back, p->gc, 0, 0, p->w, p->h, 0, 0);

	const char *hint = p->circle ? "按住圓心並拖曳出半徑，放開完成 · Esc 或右鍵取消"
	                             : "點一下取得座標 · Esc 或右鍵取消";
	pkPill(p, (p->w - pkTextWidth(p, hint) - 20) / 2, 16, hint);

	int cx = p->mx, cy = p->my;
	if (p->circle && p->pressed) {
		cx = p->px; cy = p->py;
		int r = (int)lround(hypot(p->mx - p->px, p->my - p->py));
		if (r > 0) {
			XSetForeground(d, p->gc, p->blue);
			XSetFillStyle(d, p->gc, FillStippled);
			XSetStipple(d, p->gc, p->stipple);
			XFillArc(d, p->back, p->gc, cx - r, cy - r, 2 * r, 2 * r, 0, 360 * 64);
			XSetFillStyle(d, p->gc, FillSolid);
			XSetLineAttributes(d, p->gc, 4, LineSolid, CapRound, JoinRound);
			XSetForeground(d, p->gc, p->light);
			XDrawArc(d, p->back, p->gc, cx - r, cy - r, 2 * r, 2 * r, 0, 360 * 64);
			XSetLineAttributes(d, p->gc, 2, LineSolid, CapRound, JoinRound);
			XSetForeground(d, p->gc, p->blue);
			XDrawArc(d, p->back, p->gc, cx - r, cy - r, 2 * r, 2 * r, 0, 360 * 64);
			pkLine(p, cx, cy, p->mx, p->my);
		}
		XSetForeground(d, p->gc, p->blue);
		XFillArc(d, p->back, p->gc, cx - 4, cy - 4, 8, 8, 0, 360 * 64);
		snprintf(buf, sizeof buf, "圓心 %d, %d · r=%d", p->ox + p->px, p->oy + p->py, r);
	} else {
		snprintf(buf, sizeof buf, "%d, %d", p->ox + p->mx, p->oy + p->my);
	}

	// Full-screen crosshair with a gap at the pointer.
	int g = 6;
	pkLine(p, 0, p->my, p->mx - g, p->my);
	pkLine(p, p->mx + g, p->my, p->w, p->my);
	pkLine(p, p->mx, 0, p->mx, p->my - g);
	pkLine(p, p->mx, p->my + g, p->mx, p->h);

	int lx = p->mx + 24, ly = p->my + 24;
	if (lx + 124 > p->w) lx = p->mx - 24 - 124;
	if (ly + 124 + 34 > p->h) ly = p->my - 24 - 124 - 34;
	pkLoupe(p, lx, ly);
	pkPill(p, lx, ly + 124, buf);

	XCopyArea(d, p->back, p->win, p->gc, 0, 0, p->w, p->h, 0, 0);
	XFlush(d);
}

static long pkNowMs(void) {
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return ts.tv_sec * 1000L + ts.tv_nsec / 1000000L;
}

static void pkClose(csPicker *p) {
	if (p->dpy == NULL) return;
	XUngrabPointer(p->dpy, CurrentTime);
	XUngrabKeyboard(p->dpy, CurrentTime);
	if (p->xd) XftDrawDestroy(p->xd);
	if (p->font) XftFontClose(p->dpy, p->font);
	if (p->win) XDestroyWindow(p->dpy, p->win);
	if (p->shot) XFreePixmap(p->dpy, p->shot);
	if (p->back) XFreePixmap(p->dpy, p->back);
	if (p->stipple) XFreePixmap(p->dpy, p->stipple);
	if (p->gc) XFreeGC(p->dpy, p->gc);
	if (p->img) XDestroyImage(p->img);
	XSync(p->dpy, False);
	XCloseDisplay(p->dpy);
	p->dpy = NULL;
}

// csPickRun returns 1 when picked, 0 when canceled or timed out, and a
// negative code on error. out: [0..1] press pixel, [2..3] release pixel,
// [4..5] layer origin on the root, [6] screenshot width, [7] layer width.
static int csPickRun(int circle, int timeoutMs, int *out) {
	csPicker pk;
	csPicker *p = &pk;
	memset(p, 0, sizeof pk);
	p->circle = circle;
	p->dpy = XOpenDisplay(NULL);
	if (p->dpy == NULL) return -1;
	Display *d = p->dpy;
	p->screen = DefaultScreen(d);
	p->root = RootWindow(d, p->screen);
	p->visual = DefaultVisual(d, p->screen);
	p->cmap = DefaultColormap(d, p->screen);
	p->depth = DefaultDepth(d, p->screen);
	p->w = DisplayWidth(d, p->screen);
	p->h = DisplayHeight(d, p->screen);

	p->img = XGetImage(d, p->root, 0, 0, p->w, p->h, AllPlanes, ZPixmap);
	if (p->img == NULL) { pkClose(p); return -2; }

	p->blue = pkRGB(p, 0x0A, 0x84, 0xFF);
	p->ink = pkRGB(p, 0x1C, 0x1C, 0x1E);
	p->light = pkRGB(p, 0xFF, 0xFF, 0xFF);
	p->dark = pkRGB(p, 0x10, 0x10, 0x14);

	XSetWindowAttributes a;
	memset(&a, 0, sizeof a);
	a.override_redirect = True;
	a.background_pixmap = None;
	a.event_mask = ExposureMask | KeyPressMask | ButtonPressMask | ButtonReleaseMask |
		PointerMotionMask | StructureNotifyMask;
	a.cursor = XCreateFontCursor(d, XC_crosshair);
	p->win = XCreateWindow(d, p->root, 0, 0, p->w, p->h, 0, p->depth, InputOutput, p->visual,
		CWOverrideRedirect | CWBackPixmap | CWEventMask | CWCursor, &a);
	XStoreName(d, p->win, "click-spirit-picker");
	p->gc = XCreateGC(d, p->win, 0, NULL);
	p->shot = XCreatePixmap(d, p->win, p->w, p->h, p->depth);
	p->back = XCreatePixmap(d, p->win, p->w, p->h, p->depth);
	XPutImage(d, p->shot, p->gc, p->img, 0, 0, 0, 0, p->w, p->h);
	static char bits[] = {0x01, 0x02};
	p->stipple = XCreateBitmapFromData(d, p->win, bits, 2, 2);
	p->xd = XftDrawCreate(d, p->back, p->visual, p->cmap);
	p->font = XftFontOpenName(d, p->screen, "sans-serif:lang=zh-tw:weight=bold:pixelsize=15");
	if (p->font == NULL) p->font = XftFontOpenName(d, p->screen, "sans-serif:pixelsize=15");
	XRenderColor wc = {0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF};
	XftColorAllocValue(d, p->visual, p->cmap, &wc, &p->white);

	XMapRaised(d, p->win);
	XEvent ev;
	for (int i = 0; i < 200; i++) {
		if (XCheckTypedWindowEvent(d, p->win, MapNotify, &ev)) break;
		usleep(5000);
	}
	int grabbed = 0;
	for (int i = 0; i < 100 && !grabbed; i++) {
		grabbed = XGrabPointer(d, p->win, False, ButtonPressMask | ButtonReleaseMask | PointerMotionMask,
			GrabModeAsync, GrabModeAsync, p->win, a.cursor, CurrentTime) == GrabSuccess;
		if (!grabbed) usleep(10000);
	}
	if (!grabbed) { pkClose(p); return -3; }
	for (int i = 0; i < 100; i++) {
		if (XGrabKeyboard(d, p->win, False, GrabModeAsync, GrabModeAsync, CurrentTime) == GrabSuccess) break;
		usleep(10000);
	}
	Window child;
	XTranslateCoordinates(d, p->win, p->root, 0, 0, &p->ox, &p->oy, &child);
	Window rr, cr;
	int rx, ry, wx, wy;
	unsigned int mask;
	if (XQueryPointer(d, p->win, &rr, &cr, &rx, &ry, &wx, &wy, &mask)) { p->mx = wx; p->my = wy; }
	pkRedraw(p);

	long deadline = pkNowMs() + timeoutMs;
	int rc = 0;
	int fd = ConnectionNumber(d);
	for (;;) {
		if (pkNowMs() > deadline) { rc = 0; break; }
		if (XPending(d) == 0) {
			struct pollfd pf = {fd, POLLIN, 0};
			poll(&pf, 1, 50);
			continue;
		}
		XNextEvent(d, &ev);
		int dirty = 0, done = 0;
		switch (ev.type) {
		case Expose:
			dirty = 1;
			break;
		case MotionNotify:
			while (XCheckTypedWindowEvent(d, p->win, MotionNotify, &ev)) {}
			p->mx = ev.xmotion.x; p->my = ev.xmotion.y;
			dirty = 1;
			break;
		case ButtonPress:
			if (ev.xbutton.button == 3) { rc = 0; done = 1; break; }
			if (ev.xbutton.button != 1) break;
			p->mx = ev.xbutton.x; p->my = ev.xbutton.y;
			p->pressed = 1; p->px = p->mx; p->py = p->my;
			dirty = 1;
			break;
		case ButtonRelease:
			if (ev.xbutton.button != 1) break;
			p->mx = ev.xbutton.x; p->my = ev.xbutton.y;
			if (!p->pressed) { p->px = p->mx; p->py = p->my; }
			if (p->circle && lround(hypot(p->mx - p->px, p->my - p->py)) < 1) {
				p->pressed = 0; // a click without a drag is not a circle yet
				dirty = 1;
				break;
			}
			out[0] = p->px; out[1] = p->py; out[2] = p->mx; out[3] = p->my;
			rc = 1; done = 1;
			break;
		case KeyPress: {
			KeySym ks = XLookupKeysym(&ev.xkey, 0);
			if (ks == XK_Escape) { rc = 0; done = 1; }
			break;
		}
		}
		if (done) break;
		if (dirty) pkRedraw(p);
	}
	out[4] = p->ox; out[5] = p->oy; out[6] = p->img->width; out[7] = p->w;
	pkClose(p);
	return rc;
}

// Test helpers: drive the picker through XTest on the private test display.
static int pkFake(int kind, int a, int b) {
	Display *d = XOpenDisplay(NULL);
	if (d == NULL) return -1;
	switch (kind) {
	case 0: XTestFakeMotionEvent(d, -1, a, b, 0); break;
	case 1: XTestFakeButtonEvent(d, (unsigned)a, b ? True : False, 0); break;
	case 2: {
		KeyCode kc = XKeysymToKeycode(d, XK_Escape);
		XTestFakeKeyEvent(d, kc, True, 0);
		XTestFakeKeyEvent(d, kc, False, 0);
		break;
	}
	}
	XSync(d, False);
	XCloseDisplay(d);
	return 0;
}

static int pkMapped(void) {
	Display *d = XOpenDisplay(NULL);
	if (d == NULL) return -1;
	Window root = DefaultRootWindow(d), r, parent, *kids = NULL;
	unsigned int n = 0;
	int found = 0;
	if (XQueryTree(d, root, &r, &parent, &kids, &n)) {
		for (unsigned int i = 0; i < n; i++) {
			char *name = NULL;
			XWindowAttributes wa;
			if (XFetchName(d, kids[i], &name) && name) {
				if (strcmp(name, "click-spirit-picker") == 0 && XGetWindowAttributes(d, kids[i], &wa) && wa.map_state == IsViewable) found = 1;
				XFree(name);
			}
		}
		if (kids) XFree(kids);
	}
	XCloseDisplay(d);
	return found;
}
*/
import "C"

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

// pickTimeout cancels a forgotten picker so it cannot hold the grab forever.
const pickTimeout = 2 * time.Minute

type x11Picker struct {
	mu sync.Mutex
}

func (p *x11Picker) Pick(mode core.PickMode) (core.PickResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	circle := C.int(0)
	if mode == core.PickCircle {
		circle = 1
	}
	var out [8]C.int
	rc := C.csPickRun(circle, C.int(pickTimeout/time.Millisecond), &out[0])
	switch {
	case rc == 0:
		return core.PickResult{}, core.ErrPickCanceled
	case rc < 0:
		return core.PickResult{}, fmt.Errorf("X11 screen picker failed (code %d): cannot open the display, capture the screen, or grab the pointer", int(rc))
	}
	return pickFromLayer(mode, [8]int{
		int(out[0]), int(out[1]), int(out[2]), int(out[3]),
		int(out[4]), int(out[5]), int(out[6]), int(out[7]),
	}), nil
}

// pickFromLayer converts layer pixels reported by csPickRun to clicker
// coordinates. On X11 the layer covers the root at (0,0) and the screenshot
// is 1:1, but the mapping still goes through ScreenGeometry.
func pickFromLayer(mode core.PickMode, o [8]int) core.PickResult {
	g := core.ScreenGeometry{OriginX: o[4], OriginY: o[5], Scale: 1}
	if o[6] > 0 && o[7] > 0 {
		g.Scale = float64(o[6]) / float64(o[7])
	}
	if mode == core.PickCircle {
		pcx, pcy, pr := core.CircleFromDrag(o[0], o[1], o[2], o[3])
		x, y := g.ToScreen(float64(pcx), float64(pcy))
		return core.PickResult{Mode: mode, X: x, Y: y, R: g.RadiusToScreen(float64(pr))}
	}
	x, y := g.ToScreen(float64(o[2]), float64(o[3]))
	return core.PickResult{Mode: mode, X: x, Y: y}
}

// NewScreenPicker returns the on-screen picker for the active backend.
// Only X11 has one on Linux; uinput (Wayland) cannot capture the screen.
func NewScreenPicker() (core.ScreenPicker, error) {
	if Name() != "x11" {
		return nil, fmt.Errorf("screen picker needs the X11 backend: %w", ErrUnsupported)
	}
	return &x11Picker{}, nil
}

func pickFakeMotion(x, y int)         { C.pkFake(0, C.int(x), C.int(y)) }
func pickFakeButton(b int, down bool) { C.pkFake(1, C.int(b), boolInt(down)) }
func pickFakeEscape()                 { C.pkFake(2, 0, 0) }
func pickerMapped() bool              { return C.pkMapped() == 1 }

func boolInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}
